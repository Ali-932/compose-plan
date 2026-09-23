package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Ali-932/compose-plan/internal/engine"
	"github.com/Ali-932/compose-plan/internal/history"
)

// Unit tests: go test -short .     (no Docker needed)
// Everything:  go test .           (also runs TestApplyAndRollback against Docker)

// ---------- helpers ----------

func TestFindEntry(t *testing.T) {
	entries := []history.Entry{{Seq: 1}, {Seq: 2, Summary: "second"}}

	if e, err := findEntry(2, entries); err != nil || e.Summary != "second" {
		t.Errorf("findEntry(2) = %+v, %v", e, err)
	}
	if _, err := findEntry(9, entries); err == nil {
		t.Error("findEntry(9) should fail")
	}
}

func TestSortedKeys(t *testing.T) {
	if got := sortedKeys(map[string]string{"web": "", "db": "", "cache": ""}); strings.Join(got, ",") != "cache,db,web" {
		t.Errorf("got %v", got)
	}
	if got := sortedKeys(map[string]string{}); len(got) != 0 {
		t.Errorf("empty map gave %v", got)
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

// ---------- commands that stop before Docker ----------

func TestBadArguments(t *testing.T) {
	ctx := context.Background()

	for _, err := range []error{
		runDiff(ctx, nil, nil, "", "x", "2"),
		runDiff(ctx, nil, nil, "", "1", ""),
		rollBack(ctx, nil, nil, "", "x"),
		rollBack(ctx, nil, nil, "", ""),
	} {
		if err == nil || !strings.Contains(err.Error(), "usage") {
			t.Errorf("want a usage error, got %v", err)
		}
	}
}

func TestHistoryAndDiff(t *testing.T) {
	ctx := context.Background()
	file := writeCompose(t, t.TempDir(), "nginx")

	out := captureStdout(t, func() error { return runHistory(ctx, []string{file}, nil, "") })
	if !strings.Contains(out, "no deploys recorded yet") {
		t.Errorf("empty history printed:\n%s", out)
	}

	record(t, file, map[string]string{"web": "sha256:aaa"})
	record(t, file, map[string]string{"web": "sha256:bbb"})

	out = captureStdout(t, func() error { return runHistory(ctx, []string{file}, nil, "") })
	if strings.Index(out, "#2") > strings.Index(out, "#1") {
		t.Errorf("history must list newest first:\n%s", out)
	}

	out = captureStdout(t, func() error { return runDiff(ctx, []string{file}, nil, "", "1", "2") })
	if !strings.Contains(out, "sha256:aaa -> sha256:bbb") {
		t.Errorf("diff must show the web change:\n%s", out)
	}

	if err := runDiff(ctx, []string{file}, nil, "", "1", "9"); err == nil {
		t.Error("diff to a missing entry must fail")
	}
	if err := rollBack(ctx, []string{file}, nil, "", "9"); err == nil {
		t.Error("rollback to a missing entry must fail")
	}
}

// ---------- the whole cycle against real Docker ----------

func TestApplyAndRollback(t *testing.T) {
	d := newDeployTest(t)

	d.useImage("nginx:1.26-alpine")
	d.apply()
	d.expectEntries(1)
	first := d.entries()[0].Services["web"]
	d.expectRunning(first)

	d.useImage("nginx:1.27-alpine")
	d.apply()
	d.expectEntries(2)
	if d.running() == first {
		t.Fatal("second apply did not change the image")
	}

	d.apply() // nothing changed
	d.expectEntries(2)

	d.rollback(1)
	d.expectEntries(3)
	d.expectRunning(first)
	if s := d.entries()[2].Summary; !strings.Contains(s, "rollback") {
		t.Errorf("entry #3 summary = %q, want a rollback", s)
	}

	d.rollback(1) // already there
	d.expectEntries(3)
}

// deployTest is a throwaway Compose project named "cptest" in a temp dir.
type deployTest struct {
	t    *testing.T
	file string
}

const testProject = "cptest"

func newDeployTest(t *testing.T) *deployTest {
	if testing.Short() {
		t.Skip("needs Docker; skipped with -short")
	}
	if exec.Command("docker", "version").Run() != nil {
		t.Skip("Docker is not running")
	}
	t.Cleanup(func() { exec.Command("docker", "compose", "-p", testProject, "down").Run() })
	return &deployTest{t: t, file: filepath.Join(t.TempDir(), "compose.yaml")}
}

func (d *deployTest) useImage(image string) { writeCompose(d.t, filepath.Dir(d.file), image) }

func (d *deployTest) apply() {
	d.t.Helper()
	if err := runApply(context.Background(), applyOptions{Files: []string{d.file}, Name: testProject}); err != nil {
		d.t.Fatal(err)
	}
}

func (d *deployTest) rollback(seq int) {
	d.t.Helper()
	if err := rollBack(context.Background(), []string{d.file}, nil, testProject, strconv.Itoa(seq)); err != nil {
		d.t.Fatal(err)
	}
}

func (d *deployTest) entries() []history.Entry {
	d.t.Helper()
	entries, err := history.Read(historyPath(d.file))
	if err != nil {
		d.t.Fatal(err)
	}
	return entries
}

// running returns the fingerprint of the image web runs now, the same value apply records.
func (d *deployTest) running() string {
	d.t.Helper()
	services, err := engine.Running(context.Background(), testProject)
	if err != nil || len(services) != 1 {
		d.t.Fatalf("running services: %v, %v", services, err)
	}
	return services[0].Fingerprint()
}

func (d *deployTest) expectEntries(n int) {
	d.t.Helper()
	if got := len(d.entries()); got != n {
		d.t.Fatalf("want %d history entries, got %d", n, got)
	}
}

func (d *deployTest) expectRunning(fingerprint string) {
	d.t.Helper()
	if got := d.running(); got != fingerprint {
		d.t.Fatalf("web runs %s, want %s", got, fingerprint)
	}
}

// ---------- small shared helpers ----------

// writeCompose writes a one-service compose file (service "web") into dir.
func writeCompose(t *testing.T, dir, image string) string {
	t.Helper()
	file := filepath.Join(dir, "compose.yaml")
	if err := os.WriteFile(file, []byte("services:\n  web:\n    image: "+image+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return file
}

func historyPath(composeFile string) string {
	return filepath.Join(filepath.Dir(composeFile), ".compose-plan", "history.jsonl")
}

// record appends a history entry as apply would, without deploying anything.
func record(t *testing.T, composeFile string, services map[string]string) {
	t.Helper()
	if _, err := history.Append(history.Entry{User: "test", Services: services}, historyPath(composeFile)); err != nil {
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
