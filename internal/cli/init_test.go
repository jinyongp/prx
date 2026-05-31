package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"prx/internal/config"
)

func TestInitCreatesValidConfig(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var out, errb bytes.Buffer
	if code := Init([]string{"--name", "demo"}, &out, &errb); code != ExitOK {
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
	if code := Init(nil, &out, &errb); code != ExitOK {
		t.Fatalf("first Init exit = %d", code)
	}
	if code := Init(nil, &out, &errb); code != ExitError {
		t.Fatalf("second Init exit = %d, want error (no clobber)", code)
	}
	if code := Init([]string{"--force"}, &out, &errb); code != ExitOK {
		t.Fatalf("Init --force exit = %d", code)
	}
}

func TestInitJSON(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var out, errb bytes.Buffer
	if code := Init([]string{"--json", "--name", "My App"}, &out, &errb); code != ExitOK {
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
