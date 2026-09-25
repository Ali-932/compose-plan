package plan

import (
	"bytes"
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	var buf bytes.Buffer
	Render(&buf, []Change{
		{Service: "worker", Action: "update", Reason: []string{"first reason", "second reason"}, Notes: []string{"a note"}},
		{Service: "db", Action: "no change"},
	})
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("want 4 lines, got %d:\n%s", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[0], "worker") || !strings.HasSuffix(lines[0], "first reason") {
		t.Errorf("first line: %q", lines[0])
	}
	// Continuation lines start with padding and line up under the first reason.
	if col := strings.Index(lines[0], "first reason"); strings.Index(lines[1], "second reason") != col || strings.Index(lines[2], "a note") != col {
		t.Errorf("continuation lines not aligned:\n%s", buf.String())
	}
	if strings.Join(strings.Fields(lines[3]), " ") != "db no change" {
		t.Errorf("no change line: %q", lines[3])
	}
}

func TestShortDigest(t *testing.T) {
	if got := ShortDigest("sha256:0123456789abcdef0123"); got != "sha256:0123456789ab..." {
		t.Errorf("got %q", got)
	}
	if got := ShortDigest("short"); got != "short" {
		t.Errorf("non-digest must pass through, got %q", got)
	}
}

func TestFingerprint(t *testing.T) {
	if got := (Service{Digest: "d", ImageID: "i"}).Fingerprint(); got != "d" {
		t.Errorf("registry digest wins, got %q", got)
	}
	if got := (Service{ImageID: "i"}).Fingerprint(); got != "i" {
		t.Errorf("local build falls back to image ID, got %q", got)
	}
}

func TestConfigChanges(t *testing.T) {
	old := map[string]any{"command": "a", "ports": []any{"80"}, "image": "x:1", "user": "root"}
	cur := map[string]any{"command": "b", "ports": []any{"80"}, "image": "x:2", "volumes": []any{"data:/d"}}

	got := strings.Join(configChanges(old, cur), "\n")
	for _, want := range []string{`command: "a" -> "b"`, `user: "root" -> none`, `volumes: none -> ["data:/d"]`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "ports") || strings.Contains(got, "image") {
		t.Errorf("unchanged ports and live-checked image must not be listed:\n%s", got)
	}
}
