package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Ali-932/compose-plan/internal/compose"
	"github.com/Ali-932/compose-plan/internal/engine"
	"github.com/Ali-932/compose-plan/internal/history"
	"github.com/Ali-932/compose-plan/internal/plan"
	"github.com/Ali-932/compose-plan/internal/registry"
	"github.com/spf13/pflag"
)

// External reads are replaceable so command tests never need a live daemon or registry.
var (
	runningServices = engine.Running
	hasImage        = engine.HasImage
	resolveImage    = registry.Resolve
)

const usage = `usage: compose-plan [-f file]... [-p name] [--env-file file]... <command>

  plan         show what apply would change
  apply        deploy with docker compose and record a history entry
  history      list recorded deploys, newest first
  diff A B     compare the images of two history entries
  rollback N   deploy the images of history entry N again
`

func main() {
	var files, envFiles []string
	var name string
	pflag.StringSliceVarP(&files, "f", "f", []string{}, "compose file (repeatable)")
	pflag.StringSliceVar(&envFiles, "env-file", []string{}, "variables file (repeatable)")
	pflag.StringVarP(&name, "p", "p", "", "project name")
	pflag.Parse()

	ctx := context.Background()
	var err error
	switch pflag.Arg(0) {
	case "plan":
		_, _, err = runPlan(ctx, files, envFiles, name)
	case "apply":
		err = runApply(ctx, applyOptions{Files: files, EnvFiles: envFiles, Name: name})
	case "history":
		err = runHistory(ctx, files, envFiles, name)
	case "diff":
		err = runDiff(ctx, files, envFiles, name, pflag.Arg(1), pflag.Arg(2))
	case "rollback":
		err = rollBack(ctx, files, envFiles, name, pflag.Arg(1))
	default:
		_, _ = fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "compose-plan:", err)
		os.Exit(1)
	}
}

// runPlan loads the file, reads what is running, prints the diff, and returns
// the project and the names of the services that would change.
func runPlan(ctx context.Context, files, envFiles []string, name string) (compose.Project, map[string]bool, error) {
	project, err := compose.Load(ctx, files, envFiles, name)
	if err != nil {
		return project, nil, err
	}
	running, err := runningServices(ctx, project.Name)
	if err != nil {
		return project, nil, err
	}
	changes, _ := plan.Diff(ctx, project.Services, running, resolveImage)
	plan.Render(os.Stdout, changes)

	changed := make(map[string]bool)
	for _, c := range changes {
		if c.Action != "no change" {
			changed[c.Service] = c.Pull
			//changed = append(changed, c.Service)
		}
	}
	return project, changed, nil
}

type applyOptions struct {
	Files    []string
	EnvFiles []string
	Name     string
	Summary  string
}

func runApply(ctx context.Context, opts applyOptions) error {
	project, changed, err := runPlan(ctx, opts.Files, opts.EnvFiles, opts.Name)
	if err != nil {
		return err
	}
	if len(changed) == 0 {
		fmt.Println("nothing to deploy")
		return nil
	}

	args := []string{"compose"}
	for _, f := range opts.Files {
		args = append(args, "-f", f)
	}
	for _, e := range opts.EnvFiles {
		args = append(args, "--env-file", e)
	}
	if opts.Name != "" {
		args = append(args, "-p", opts.Name)
	}
	var pullServices []string
	var servicesChanged []string
	for service, needPull := range changed {
		servicesChanged = append(servicesChanged, service)
		if needPull {
			pullServices = append(pullServices, service)
		}
	}
	sort.Strings(servicesChanged)

	if len(pullServices) > 0 {
		//pullArg := strings.Join(pullServices, " ")
		pullDockerArgs := append(append([]string{}, args...), "pull")
		pullDockerArgs = append(pullDockerArgs, pullServices...)
		cmd := exec.CommandContext(ctx, "docker", pullDockerArgs...)

		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("docker pull failure: %w", err)
		}

	}

	args = append(args, "up", "-d", "--remove-orphans")

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose up: %w", err)
	}

	// check what is running after docker compose up to get the snapshot
	running, err := runningServices(ctx, project.Name)
	if err != nil {
		return err
	}
	digests := map[string]string{}
	for _, s := range running {
		id := s.Digest
		if id == "" {
			id = s.ImageID // local build: no registry digest, the daemon ID is the fingerprint
		}
		digests[s.Name] = id
	}

	commit := ""
	if out, err := exec.Command("git", "-C", project.WorkingDir, "rev-parse", "--short", "HEAD").Output(); err == nil {
		commit = strings.TrimSpace(string(out)) // git returns a /n at the end
	}
	entrySummary := strings.Join(servicesChanged, ", ")
	if opts.Summary != "" {
		entrySummary = opts.Summary
	}
	entry, err := history.Append(history.Entry{
		Time:     time.Now(),
		User:     os.Getenv("USER"),
		Commit:   commit,
		Summary:  entrySummary,
		Services: digests,
	}, filepath.Join(project.WorkingDir, ".compose-plan", "history.jsonl"))
	if err != nil {
		return err
	}
	fmt.Printf("deployed entry #%d  %s  %s  %s  %d services\n",
		entry.Seq, entry.Time.Format("2006-01-02 15:04"), entry.User, entry.Commit, len(entry.Services))
	return nil
}

func runHistory(ctx context.Context, files, envFiles []string, name string) error {
	project, err := compose.Load(ctx, files, envFiles, name)
	if err != nil {
		return err
	}

	entries, err := history.Read(filepath.Join(project.WorkingDir, ".compose-plan", "history.jsonl"))
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		fmt.Println("no deploys recorded yet")
		return nil
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", fmt.Sprintf("#%d", e.Seq), e.Time.Format("Jan _2 15:04"), e.User, e.Commit, e.Summary)
	}

	tw.Flush()
	return nil
}

func runDiff(ctx context.Context, files, envFiles []string, name string, a, b string) error {
	aSeq, errA := strconv.Atoi(a)
	bSeq, errB := strconv.Atoi(b)
	if errA != nil || errB != nil {
		return fmt.Errorf("usage: compose-plan diff <entry> <entry>")
	}
	project, err := compose.Load(ctx, files, envFiles, name)
	if err != nil {
		return err
	}
	entries, err := history.Read(filepath.Join(project.WorkingDir, ".compose-plan", "history.jsonl"))
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("no deploys recorded yet")
		return nil
	}
	aEntry, err := findEntry(aSeq, entries)
	if err != nil {
		return err
	}
	bEntry, err := findEntry(bSeq, entries)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	for svc, digest := range aEntry.Services {
		if BDigest, ok := bEntry.Services[svc]; !ok {
			fmt.Fprintf(tw, "%s\tremoved\t%s\n", svc, plan.ShortDigest(digest))
		} else if digest != BDigest {
			fmt.Fprintf(tw, "%s\timage\t%s -> %s\n", svc, plan.ShortDigest(digest), plan.ShortDigest(BDigest))
		}
	}
	for svc, digest := range bEntry.Services {
		if _, ok := aEntry.Services[svc]; !ok {
			fmt.Fprintf(tw, "%s\tadded\t%s\n", svc, plan.ShortDigest(digest))
		}
	}
	return tw.Flush()

}

func rollBack(ctx context.Context, files, envFiles []string, name string, a string) error {
	aSeq, err := strconv.Atoi(a)
	if err != nil {
		return fmt.Errorf("usage: compose-plan rollback <entry>")
	}
	project, err := compose.Load(ctx, files, envFiles, name)
	if err != nil {
		return err
	}
	entries, err := history.Read(filepath.Join(project.WorkingDir, ".compose-plan", "history.jsonl"))
	if err != nil {
		return err
	}
	aEntry, err := findEntry(aSeq, entries)
	if err != nil {
		return err
	}
	services, err := runningServices(ctx, project.Name)
	if err != nil {
		return err
	}

	runningServices := make(map[string]string)
	needChange := make(map[string]string)
	for _, svc := range services {
		runningServices[svc.Name] = svc.Fingerprint()
	}
	imageOf := make(map[string]string)
	for _, svc := range project.Services {
		imageOf[svc.Name] = svc.Image
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	for _, svc := range sortedKeys(aEntry.Services) {
		digest := aEntry.Services[svc]
		if _, inFile := imageOf[svc]; !inFile { // if the project is not running a instance in the entry
			fmt.Fprintf(tw, "%s\tskipped\tin entry #%d but not in the compose file.\n", svc, aSeq)
			continue
		}
		if cur, ok := runningServices[svc]; !ok {
			fmt.Fprintf(tw, "%s\tcreate\t%s\n", svc, plan.ShortDigest(digest))
			needChange[svc] = digest
		} else if cur != digest {
			fmt.Fprintf(tw, "%s\timage\t%s -> %s\n", svc, plan.ShortDigest(cur), plan.ShortDigest(digest))
			needChange[svc] = digest
		}
	}
	tw.Flush()

	if len(needChange) == 0 {
		fmt.Printf("entry #%d is already what is running\n", aSeq)
		return nil
	}

	tmpFile, err := os.CreateTemp("", "compose-plan-rollback-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	var composeYAML strings.Builder
	composeYAML.WriteString("services:\n")
	for _, svc := range sortedKeys(needChange) {
		digest := needChange[svc]
		fullDigest := imageOf[svc] + "@" + digest
		found := hasImage(ctx, digest)
		if !found {
			_, err := resolveImage(ctx, fullDigest)
			if err != nil {
				return fmt.Errorf("%s: image from entry #%d no longer exists on this machine or the registry, cannot roll back", svc, aSeq)
			}
		}
		composeYAML.WriteString(fmt.Sprintf("  %s:\n    image: %s\n", svc, fullDigest))
	}
	if _, err := tmpFile.WriteString(composeYAML.String()); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()
	files = append(files, tmpFile.Name())
	err = runApply(ctx, applyOptions{
		Files:    files,
		EnvFiles: envFiles,
		Name:     name,
		Summary:  fmt.Sprintf("rollback to #%d", aSeq),
	})
	if err != nil {
		return err
	}
	fmt.Println("containers were restored, data was not: database migrations run since that entry are still applied")
	if head := gitHead(project.WorkingDir); aEntry.Commit != "" && aEntry.Commit != head {
		fmt.Printf("config at #%d was commit %s, current tree is %s; to restore it: git checkout %s && compose-plan apply\n", aSeq, aEntry.Commit, head, aEntry.Commit)
	}
	return nil
}

func findEntry(entrySeq int, Entries []history.Entry) (*history.Entry, error) {
	for _, entry := range Entries {
		if entry.Seq == entrySeq {
			return &entry, nil
		}
	}
	return &history.Entry{}, fmt.Errorf("no entry with seq %d", entrySeq)

}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
func gitHead(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
