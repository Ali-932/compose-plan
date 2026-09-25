package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/Ali-932/compose-plan/internal/engine"
	"github.com/Ali-932/compose-plan/internal/history"
)

// RunApply deploys with docker compose and records a history entry.
// summary overrides the entry's summary when not empty; rollback uses it.
func RunApply(ctx context.Context, opts Options, summary string) error {
	project, changed, err := RunPlan(ctx, opts)
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
		digests[s.Name] = s.Fingerprint()
	}

	if summary == "" {
		summary = strings.Join(servicesChanged, ", ")
	}
	entry, err := history.Append(history.Entry{
		Time:     time.Now(),
		User:     os.Getenv("USER"),
		Commit:   gitHead(project.WorkingDir),
		Summary:  summary,
		Services: digests,
	}, historyPath(project))
	if err != nil {
		return err
	}
	fmt.Printf("deployed entry #%d  %s  %s  %s  %d services\n",
		entry.Seq, entry.Time.Format("2006-01-02 15:04"), entry.User, entry.Commit, len(entry.Services))
	return nil
}
