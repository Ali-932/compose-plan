// Package compose renders compose files the way Docker Compose does, using
// github.com/compose-spec/compose-go/v2, so the plan compares what Docker
// will actually run: stacked -f files, .env, --env-file, ${VAR} interpolation.
package compose

import (
	"context"
	"sort"

	"github.com/Ali-932/compose-plan/internal/plan"
	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/types"
)

type Project struct {
	Name       string
	WorkingDir string
	Files      []string // the compose files actually loaded, found or given with -f
	Services   []plan.Service
}

func Load(ctx context.Context, files, envFiles []string, projectName string) (Project, error) {
	opts := []cli.ProjectOptionsFn{
		cli.WithOsEnv,                 // shell vars first: they win over .env
		cli.WithEnvFiles(envFiles...), // --env-file list, empty means ".env"
		cli.WithDotEnv,                // now read those files
	}
	if len(files) == 0 {
		opts = append(opts, cli.WithDefaultConfigPath) // look for compose.yaml like the CLI
	}
	if projectName != "" {
		opts = append(opts, cli.WithName(projectName))
	}

	po, err := cli.NewProjectOptions(files, opts...)
	if err != nil {
		return Project{}, err
	}
	p, err := po.LoadProject(ctx)
	if err != nil {
		return Project{}, err
	}

	return Project{Name: p.Name, WorkingDir: p.WorkingDir, Files: p.ComposeFiles, Services: services(p)}, nil
}

func services(p *types.Project) []plan.Service {
	var out []plan.Service
	for name, s := range p.Services {
		mem := int64(s.MemLimit)
		if mem == 0 && s.Deploy != nil && s.Deploy.Resources.Limits != nil {
			mem = int64(s.Deploy.Resources.Limits.MemoryBytes)
		}
		env := map[string]string{}
		for k, v := range s.Environment {
			if v != nil {
				env[k] = *v
			}
		}
		out = append(out, plan.Service{Name: name, Image: s.Image, MemLimit: mem, Env: env, Build: s.Build != nil})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
