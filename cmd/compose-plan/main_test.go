package main

import (
	"path/filepath"
	"testing"

	"github.com/Ali-932/compose-plan/internal/cli"
)

func TestCheckInput(t *testing.T) {
	missing := cli.Options{Files: []string{filepath.Join(t.TempDir(), "nope.yaml")}}

	bad := []struct {
		name string
		cmd  string
		args []string
		opts cli.Options
	}{
		{"no command", "", nil, cli.Options{}},
		{"unknown command", "deploy", nil, cli.Options{}},
		{"plan with an argument", "plan", []string{"1"}, cli.Options{}},
		{"diff with one entry", "diff", []string{"1"}, cli.Options{}},
		{"rollback with two entries", "rollback", []string{"1", "2"}, cli.Options{}},
		{"entry not a number", "rollback", []string{"x"}, cli.Options{}},
		{"entry zero", "rollback", []string{"0"}, cli.Options{}},
		{"compose file missing", "plan", nil, missing},
	}
	for _, c := range bad {
		if _, err := checkInput(c.cmd, c.args, c.opts); err == nil {
			t.Errorf("%s: want an error", c.name)
		}
	}

	entries, err := checkInput("diff", []string{"1", "2"}, cli.Options{})
	if err != nil || len(entries) != 2 || entries[0] != 1 || entries[1] != 2 {
		t.Errorf("diff 1 2: got %v, %v", entries, err)
	}
	if _, err := checkInput("plan", nil, cli.Options{}); err != nil {
		t.Errorf("plan: %v", err)
	}
}
