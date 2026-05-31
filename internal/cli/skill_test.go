package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestSkillPathMaterialises(t *testing.T) {
	isolate(t)
	var out, errb bytes.Buffer
	if code := Skill([]string{"path"}, &out, &errb); code != ExitOK {
		t.Fatalf("skill path exit = %d, stderr=%s", code, errb.String())
	}
	dest := strings.TrimSpace(out.String())
	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("materialised file missing: %v", err)
	}
	if !strings.Contains(string(b), "name: prx") {
		t.Fatalf("SKILL.md missing frontmatter:\n%s", b)
	}
}

func TestSkillPrint(t *testing.T) {
	isolate(t)
	var out, errb bytes.Buffer
	if code := Skill([]string{"print"}, &out, &errb); code != ExitOK {
		t.Fatalf("skill print exit = %d", code)
	}
	if !strings.Contains(out.String(), "# prx") {
		t.Fatalf("print output missing content")
	}
}
