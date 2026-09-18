package compose

import (
	"context"
	"testing"
)

// Run with: go test ./internal/compose/ -v
func TestLoad(t *testing.T) {
	t.Setenv("WEB_TAG", "1.27-alpine") // fills ${WEB_TAG:-latest} in testdata/compose.yaml

	p, err := Load(context.Background(), []string{"../../testdata/compose.yaml"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Name:       %s", p.Name)
	t.Logf("WorkingDir: %s", p.WorkingDir)
	for _, s := range p.Services {
		t.Logf("%-4s image=%-18s mem=%d env=%v", s.Name, s.Image, s.MemLimit, s.Env)
	}
}
