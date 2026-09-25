// Package e2e runs compose-plan commands against a real Docker daemon.
// Skipped with: go test -short ./...
package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ali-932/compose-plan/internal/cli"
	"github.com/Ali-932/compose-plan/internal/engine"
	"github.com/Ali-932/compose-plan/internal/history"
)

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

// Without -f, rollback must still deploy compose.yaml with its override on top,
// the way every other command finds compose.yaml in the current folder.
func TestRollbackWithoutFileFlag(t *testing.T) {
	d := newDeployTest(t)
	d.opts.Files = nil
	t.Chdir(d.dir)

	d.useImage("nginx:1.26-alpine")
	d.apply()
	first := d.entries()[0].Services["web"]

	d.useImage("nginx:1.27-alpine")
	d.apply()

	d.rollback(1)
	d.expectEntries(3)
	d.expectRunning(first)
}

// A stopped container is not running, so apply must start it again.
func TestApplyStartsStoppedContainer(t *testing.T) {
	d := newDeployTest(t)

	d.useImage("nginx:1.26-alpine")
	d.apply()
	d.stopWeb()

	d.apply()
	d.expectEntries(2)
	if !d.webIsRunning() {
		t.Fatal("apply left the stopped container stopped")
	}
}

// deployTest is a throwaway Compose project named "cptest" in a temp dir.
type deployTest struct {
	t    *testing.T
	dir  string // holds compose.yaml and .compose-plan/
	opts cli.Options
}

const project = "cptest"

func newDeployTest(t *testing.T) *deployTest {
	if testing.Short() {
		t.Skip("needs Docker; skipped with -short")
	}
	if exec.Command("docker", "version").Run() != nil {
		t.Skip("Docker is not running")
	}
	t.Cleanup(func() { exec.Command("docker", "compose", "-p", project, "down").Run() })
	dir := t.TempDir()
	return &deployTest{t: t, dir: dir, opts: cli.Options{Files: []string{filepath.Join(dir, "compose.yaml")}, Name: project}}
}

func (d *deployTest) useImage(image string) {
	d.t.Helper()
	if err := os.WriteFile(filepath.Join(d.dir, "compose.yaml"), []byte("services:\n  web:\n    image: "+image+"\n"), 0o644); err != nil {
		d.t.Fatal(err)
	}
}

func (d *deployTest) apply() {
	d.t.Helper()
	if err := cli.RunApply(context.Background(), d.opts, ""); err != nil {
		d.t.Fatal(err)
	}
}

func (d *deployTest) rollback(seq int) {
	d.t.Helper()
	if err := cli.RollBack(context.Background(), d.opts, seq); err != nil {
		d.t.Fatal(err)
	}
}

func (d *deployTest) entries() []history.Entry {
	d.t.Helper()
	path := filepath.Join(d.dir, ".compose-plan", "history.jsonl")
	entries, err := history.Read(path)
	if err != nil {
		d.t.Fatal(err)
	}
	return entries
}

// running returns the fingerprint of the image web runs now, the same value apply records.
func (d *deployTest) running() string {
	d.t.Helper()
	services, err := engine.Running(context.Background(), project)
	if err != nil || len(services) != 1 {
		d.t.Fatalf("running services: %v, %v", services, err)
	}
	return services[0].Fingerprint()
}

const webContainer = project + "-web-1"

func (d *deployTest) stopWeb() {
	d.t.Helper()
	if err := exec.Command("docker", "stop", webContainer).Run(); err != nil {
		d.t.Fatal(err)
	}
}

func (d *deployTest) webIsRunning() bool {
	out, _ := exec.Command("docker", "inspect", "--format", "{{.State.Running}}", webContainer).Output()
	return strings.TrimSpace(string(out)) == "true"
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
