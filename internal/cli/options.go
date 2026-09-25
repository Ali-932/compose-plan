// Package cli holds the logic of each compose-plan command. cmd/compose-plan
// parses and checks the input, then calls one exported function per command.
package cli

import (
	"path/filepath"

	"github.com/Ali-932/compose-plan/internal/compose"
)

// Options are the flags every command shares; they mirror docker compose.
type Options struct {
	Files    []string // -f, compose file, repeatable
	EnvFiles []string // --env-file, repeatable
	Name     string   // -p, project name
}

const Usage = `usage: compose-plan [-f file]... [-p name] [--env-file file]... <command>

  plan         show what apply would change
  apply        deploy with docker compose and record a history entry
  history      list recorded deploys, newest first
  diff A B     compare the images of two history entries
  rollback N   deploy the images of history entry N again
`

func historyPath(project compose.Project) string {
	return filepath.Join(project.WorkingDir, ".compose-plan", "history.jsonl")
}
