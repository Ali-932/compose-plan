package plan

import (
	"context"
	"strings"
	"testing"
)

func diff(t *testing.T, desired, running []Service) map[string]Change {
	t.Helper()
	changes, err := Diff(context.Background(), desired, running, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]Change{}
	for _, c := range changes {
		out[c.Service] = c
	}
	return out
}

func TestDiffIdenticalIsNoChange(t *testing.T) {
	same := []Service{
		{Name: "web", Image: "nginx:1.27", MemLimit: 512 << 20, Env: map[string]string{"A": "1"}},
		{Name: "db", Image: "redis:7"},
	}
	for name, c := range diff(t, same, same) {
		if c.Action != "no change" || len(c.Reason) != 0 {
			t.Errorf("%s: got %s %v, want no change", name, c.Action, c.Reason)
		}
	}
}

func TestDiffCreateAndRemove(t *testing.T) {
	got := diff(t,
		[]Service{{Name: "cache", Image: "redis:7"}},
		[]Service{{Name: "gone", Image: "old:1"}},
	)
	if got["cache"].Action != "create" {
		t.Errorf("cache: got %s, want create", got["cache"].Action)
	}
	if got["gone"].Action != "remove" {
		t.Errorf("gone: got %s, want remove", got["gone"].Action)
	}
}

func TestDiffImageTextChanged(t *testing.T) {
	c := diff(t,
		[]Service{{Name: "web", Image: "nginx:1.27"}},
		[]Service{{Name: "web", Image: "nginx:1.26"}},
	)["web"]
	if c.Action != "update" || !strings.Contains(strings.Join(c.Reason, ";"), "nginx:1.26 -> nginx:1.27") {
		t.Errorf("got %s %v", c.Action, c.Reason)
	}
}

func TestDiffImageRebuiltLocally(t *testing.T) {
	c := diff(t,
		[]Service{{Name: "api", Image: "app"}},
		[]Service{{Name: "api", Image: "app", ImageID: "sha256:old", TaggedID: "sha256:new"}},
	)["api"]
	if c.Action != "update" || !strings.Contains(strings.Join(c.Reason, ";"), "changed on this machine") {
		t.Errorf("got %s %v", c.Action, c.Reason)
	}
}

func TestDiffMemoryAndEnvGiveOneChange(t *testing.T) {
	c := diff(t,
		[]Service{{Name: "worker", Image: "app:1", MemLimit: 1 << 30, Env: map[string]string{"A": "1", "B": "2"}}},
		// PATH comes from the image, not the file, so it must be ignored.
		[]Service{{Name: "worker", Image: "app:1", MemLimit: 512 << 20, Env: map[string]string{"A": "1", "B": "old", "PATH": "/bin"}}},
	)["worker"]
	if c.Action != "update" || len(c.Reason) != 2 {
		t.Fatalf("want one update with two reasons, got %s %v", c.Action, c.Reason)
	}
	env := c.Reason[1]
	if !strings.Contains(env, "(B)") || strings.Contains(env, "old") || strings.Contains(env, "PATH") {
		t.Errorf("env reason must name only B and hide values, got %q", env)
	}
}

func TestDiffNoChangeSortsLast(t *testing.T) {
	changes, _ := Diff(context.Background(),
		[]Service{{Name: "a", Image: "x"}, {Name: "b", Image: "new"}, {Name: "c", Image: "x"}},
		[]Service{{Name: "a", Image: "x"}, {Name: "b", Image: "old"}, {Name: "c", Image: "x"}},
		nil)
	var order []string
	for _, c := range changes {
		order = append(order, c.Service)
	}
	if strings.Join(order, ",") != "b,a,c" {
		t.Errorf("got order %v, want b first then a,c", order)
	}
}
