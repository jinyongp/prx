package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderContainsCells(t *testing.T) {
	out := Render([]string{"A", "B"}, [][]string{{"x1", "y1"}, {"x2", "y2"}})
	for _, want := range []string{"A", "B", "x1", "y1", "x2", "y2"} {
		if !strings.Contains(out, want) {
			t.Fatalf("Render missing %q in:\n%s", want, out)
		}
	}
	if n := strings.Count(out, "\n"); n < 2 { // header + 2 rows
		t.Fatalf("Render too few lines (%d):\n%s", n, out)
	}
}

func TestEnabledNonFileFalse(t *testing.T) {
	if Enabled(&bytes.Buffer{}) {
		t.Fatal("Enabled must be false for a non-*os.File writer")
	}
}
