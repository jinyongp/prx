package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"prx/internal/config"
)

func TestInitCreatesValidConfig(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var out, errb bytes.Buffer
	if code := Init([]string{"-y", "--name", "demo"}, &out, &errb); code != ExitOK {
		t.Fatalf("Init exit = %d, stderr=%s", code, errb.String())
	}
	path := filepath.Join(dir, config.Filename)
	p, err := config.Load(path)
	if err != nil {
		t.Fatalf("generated prx.toml invalid: %v", err)
	}
	if p.Name != "demo" {
		t.Fatalf("project name = %q", p.Name)
	}
	svc, ok := p.Services["web"]
	if !ok || svc.Domain != "demo.localhost" {
		t.Fatalf("web service = %+v", p.Services)
	}
}

func TestInitNoClobber(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var out, errb bytes.Buffer
	if code := Init([]string{"-y"}, &out, &errb); code != ExitOK {
		t.Fatalf("first Init exit = %d", code)
	}
	if code := Init([]string{"-y"}, &out, &errb); code != ExitError {
		t.Fatalf("second Init exit = %d, want error (no clobber)", code)
	}
	if code := Init([]string{"-y", "--force"}, &out, &errb); code != ExitOK {
		t.Fatalf("Init --force exit = %d", code)
	}
}

func TestInitJSON(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var out, errb bytes.Buffer
	if code := Init([]string{"--json", "-y", "--name", "My App"}, &out, &errb); code != ExitOK {
		t.Fatalf("Init --json exit = %d", code)
	}
	var got struct {
		Project string `json:"project"`
		Created bool   `json:"created"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out.String())
	}
	if !got.Created || got.Project != "My App" {
		t.Fatalf("unexpected: %+v", got)
	}
	// "My App" must yield a valid DNS label.
	p, err := config.Load(filepath.Join(dir, config.Filename))
	if err != nil {
		t.Fatalf("invalid config from spaced name: %v", err)
	}
	if p.Services["web"].Domain != "my-app.localhost" {
		t.Fatalf("domain = %q", p.Services["web"].Domain)
	}
}

func TestInitNonInteractiveRequiresYes(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var out, errb bytes.Buffer
	if code := Init(nil, &out, &errb); code != ExitUsage {
		t.Fatalf("Init exit = %d, want usage; stderr=%s", code, errb.String())
	}
}

func TestRenderInteractiveCustomDomainSpec(t *testing.T) {
	spec := initSpec{
		ProjectName: "demo",
		Services: []initService{
			{Name: "web", Domain: "local.project.test"},
			{Name: "api", Domain: "api.local.project.test", Port: 3001},
		},
	}
	got := renderInitSpec(spec)
	path := filepath.Join(t.TempDir(), config.Filename)
	if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := config.Load(path)
	if err != nil {
		t.Fatalf("generated prx.toml invalid: %v\n%s", err, got)
	}
	if p.Services["web"].Domain != "local.project.test" {
		t.Fatalf("web domain = %q", p.Services["web"].Domain)
	}
	if p.Services["api"].Domain != "api.local.project.test" || p.Services["api"].Port != 3001 {
		t.Fatalf("api service = %+v", p.Services["api"])
	}
}
