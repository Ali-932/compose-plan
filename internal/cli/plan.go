package cli

import (
	"context"
	"os"

	"github.com/Ali-932/compose-plan/internal/compose"
	"github.com/Ali-932/compose-plan/internal/engine"
	"github.com/Ali-932/compose-plan/internal/plan"
	"github.com/Ali-932/compose-plan/internal/registry"
)

// RunPlan loads the file, reads what is running, prints the diff, and returns
// the project and the services that would change (true = needs a pull).
func RunPlan(ctx context.Context, opts Options) (compose.Project, map[string]bool, error) {
	project, err := compose.Load(ctx, opts.Files, opts.EnvFiles, opts.Name)
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
		}
	}
	return project, changed, nil
}
