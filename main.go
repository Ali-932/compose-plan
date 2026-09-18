package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Ali-932/compose-plan/internal/compose"
	"github.com/Ali-932/compose-plan/internal/engine"
	"github.com/Ali-932/compose-plan/internal/plan"
	"github.com/Ali-932/compose-plan/internal/registry"
	"github.com/spf13/pflag"
)

func main() {
	var files, envFiles []string
	var name string
	pflag.StringSliceVar(&files, "f", []string{}, "compose file (repeatable)")
	pflag.StringSliceVar(&envFiles, "env-file", []string{}, "variables file (repeatable)")
	pflag.StringVar(&name, "p", "", "project name")
	pflag.Parse()

	if pflag.Arg(0) != "plan" {
		_, _ = fmt.Fprintln(os.Stderr, "usage: compose-plan [-f file]... [-p name] plan")
		os.Exit(2)
	}

	ctx := context.Background()
	project, err := compose.Load(ctx, files, envFiles, name)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "compose-plan:", err)
		os.Exit(1)
	}
	running, err := engine.Running(ctx, project.Name)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "compose-plan:", err)
		os.Exit(1)
	}
	changes, _ := plan.Diff(ctx, project.Services, running, registry.Resolve)
	plan.Render(os.Stdout, changes)
}
