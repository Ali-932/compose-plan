package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ali-932/compose-plan/internal/compose"
	"github.com/Ali-932/compose-plan/internal/history"
)

// Run with: go test ./internal/cli/ -v   (no Docker needed)

func TestHistoryAndDiff(t *testing.T) {
	ctx := context.Background()
	opts := Options{Files: []string{writeCompose(t)}}

	out := captureStdout(t, func() error { return RunHistory(ctx, opts) })
	if !strings.Contains(out, "no deploys recorded yet") {
		t.Errorf("empty history printed:\n%s", out)
	}

	record(t, opts, map[string]string{"web": "sha256:aaa"})
	record(t, opts, map[string]string{"web": "sha256:bbb"})

	out = captureStdout(t, func() error { return RunHistory(ctx, opts) })
	if strings.Index(out, "#2") > strings.Index(out, "#1") {
		t.Errorf("history must list newest first:\n%s", out)
	}

	out = captureStdout(t, func() error { return RunDiff(ctx, opts, 1, 2) })
	if !strings.Contains(out, "sha256:aaa -> sha256:bbb") {
		t.Errorf("diff must show the web change:\n%s", out)
	}

	if err := RunDiff(ctx, opts, 1, 9); err == nil {
		t.Error("diff to a missing entry must fail")
	}
	if err := RollBack(ctx, opts, 9); err == nil {
		t.Error("rollback to a missing entry must fail")
	}
}

func TestGitHead(t *testing.T) {
	if got := gitHead(t.TempDir()); got != "" {
		t.Errorf("outside a repo: want empty, got %q", got)
	}
	if got := gitHead("."); len(got) < 7 {
		t.Errorf("inside this repo: want a commit, got %q", got)
	}
}

// writeCompose writes a one-service compose file into a temp dir.
func writeCompose(t *testing.T) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "compose.yaml")
	if err := os.WriteFile(file, []byte("services:\n  web:\n    image: nginx\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return file
}

// record appends a history entry as apply would, without deploying anything.
func record(t *testing.T, opts Options, services map[string]string) {
	t.Helper()
	path := historyPath(compose.Project{WorkingDir: filepath.Dir(opts.Files[0])})
	if _, err := history.Append(history.Entry{User: "test", Services: services}, path); err != nil {
		t.Fatal(err)
	}
}

// captureStdout runs fn and returns what it printed.
func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()
	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w
	err := fn()
	w.Close()
	os.Stdout = stdout
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
