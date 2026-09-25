package plan

import (
	"context"
	"strings"
	"testing"
)

func diff(t *testing.T, desired, running []Service) map[string]Change {
	t.Helper()
	changes := Diff(context.Background(), desired, running, nil, nil)
	out := map[string]Change{}
	for _, c := range changes {
		out[c.Service] = c
	}
	return out
}

func TestDiffIdenticalIsNoChange(t *testing.T) {
	same := []Service{
		{Name: "web", Image: "nginx:1.27"},
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
	was := map[string]any{"mem_limit": 536870912.0, "environment": map[string]any{"A": "hashA", "B": "hashB"}}
	now := map[string]any{"mem_limit": 1073741824.0, "environment": map[string]any{"A": "hashA", "B": "hashB2"}}

	c := Diff(context.Background(),
		[]Service{{Name: "worker", Image: "app:1", Config: now}},
		[]Service{{Name: "worker", Image: "app:1"}},
		map[string]map[string]any{"worker": was}, nil)[0]

	if c.Action != "update" || len(c.Reason) != 2 {
		t.Fatalf("want one update with two reasons, got %s %v", c.Action, c.Reason)
	}
	all := strings.Join(c.Reason, ";")
	if !strings.Contains(all, "(B)") || strings.Contains(all, "hashB") || !strings.Contains(all, "mem_limit") {
		t.Errorf("env reason must name only B and hide values, memory must be listed: %v", c.Reason)
	}
}

func TestDiffNoChangeSortsLast(t *testing.T) {
	changes := Diff(context.Background(),
		[]Service{{Name: "a", Image: "x"}, {Name: "b", Image: "new"}, {Name: "c", Image: "x"}},
		[]Service{{Name: "a", Image: "x"}, {Name: "b", Image: "old"}, {Name: "c", Image: "x"}},
		nil, nil)
	var order []string
	for _, c := range changes {
		order = append(order, c.Service)
	}
	if strings.Join(order, ",") != "b,a,c" {
		t.Errorf("got order %v, want b first then a,c", order)
	}
}

func TestDiffReportsRecordedConfigChanges(t *testing.T) {
	desired := []Service{{Name: "web", Image: "nginx", Config: map[string]any{"image": "nginx", "ports": []any{"8080:80"}}}}
	running := []Service{{Name: "web", Image: "nginx"}}

	changed := Diff(context.Background(), desired, running,
		map[string]map[string]any{"web": {"image": "nginx"}}, nil)
	if c := changed[0]; c.Action != "update" || !strings.Contains(strings.Join(c.Reason, ";"), "ports: none -> ") {
		t.Errorf("port added since the last deploy: got %s %v", c.Action, c.Reason)
	}

	same := Diff(context.Background(), desired, running,
		map[string]map[string]any{"web": desired[0].Config}, nil)
	if same[0].Action != "no change" {
		t.Errorf("config identical to the last deploy: got %s %v", same[0].Action, same[0].Reason)
	}
}
