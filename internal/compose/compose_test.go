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
		t.Logf("%-4s image=%s", s.Name, s.Image)
	}
}

func TestConfigHidesEnvValues(t *testing.T) {
	p, err := Load(context.Background(), []string{"../../testdata/compose.yaml"}, nil, "demo")
	if err != nil {
		t.Fatal(err)
	}
	web := p.Services[1]
	env := web.Config["environment"].(map[string]any)
	if v := env["GREETING"]; v == "hello" || len(v.(string)) != 8 {
		t.Errorf("GREETING must be stored as an 8-character hash, got %v", v)
	}
	if web.Config["mem_limit"] == nil {
		t.Error("config should carry the rest of the service, mem_limit is missing")
	}
}
