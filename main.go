package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Ali-932/compose-plan/internal/compose"
	"github.com/Ali-932/compose-plan/internal/engine"
	"github.com/Ali-932/compose-plan/internal/history"
	"github.com/Ali-932/compose-plan/internal/plan"
	"github.com/Ali-932/compose-plan/internal/registry"
	"github.com/spf13/pflag"
)

const usage = `usage: compose-plan [-f file]... [-p name] [--env-file file]... <command>

  plan    show what apply would change
  apply   deploy with docker compose and record a history entry
`

func main() {
	var files, envFiles []string
	var name string
	pflag.StringSliceVar(&files, "f", []string{}, "compose file (repeatable)")
	pflag.StringSliceVar(&envFiles, "env-file", []string{}, "variables file (repeatable)")
	pflag.StringVar(&name, "p", "", "project name")
	pflag.Parse()

	ctx := context.Background()
	var err error
	switch pflag.Arg(0) {
	case "plan":
		_, _, err = runPlan(ctx, files, envFiles, name)
	case "apply":
		err = runApply(ctx, files, envFiles, name)
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
	running, err := engine.Running(ctx, project.Name)
	if err != nil {
		return project, nil, err
	}
	changes, _ := plan.Diff(ctx, project.Services, running, registry.Resolve)
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

func runApply(ctx context.Context, files, envFiles []string, name string) error {
	project, changed, err := runPlan(ctx, files, envFiles, name)
	if err != nil {
		return err
	}
	if len(changed) == 0 {
		fmt.Println("nothing to deploy")
		return nil
	}

	args := []string{"compose"}
	for _, f := range files {
		args = append(args, "-f", f)
	}
	for _, e := range envFiles {
		args = append(args, "--env-file", e)
	}
	if name != "" {
		args = append(args, "-p", name)
	}
	var pullServices []string
	var servicesChanged []string
	for service, needPull := range changed {
		servicesChanged = append(servicesChanged, service)
		if needPull {
			pullServices = append(pullServices, service)
		}
	}
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
	running, err := engine.Running(ctx, project.Name)
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
	entry, err := history.Append(history.Entry{
		Time:     time.Now(),
		User:     os.Getenv("USER"),
		Commit:   commit,
		Summary:  strings.Join(servicesChanged, ", "),
		Services: digests,
	}, filepath.Join(project.WorkingDir, ".compose-plan", "history.jsonl"))
	if err != nil {
		return err
	}
	fmt.Printf("deployed entry #%d  %s  %s  %s  %d services\n",
		entry.Seq, entry.Time.Format("2006-01-02 15:04"), entry.User, entry.Commit, len(entry.Services))
	return nil
}
