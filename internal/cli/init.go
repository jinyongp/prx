package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"prx/internal/config"
	"prx/internal/ui"
)

// Init scaffolds a starter prx.toml in the current directory.
func Init(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "emit JSON")
	force := fs.Bool("force", false, "overwrite an existing prx.toml")
	name := fs.String("name", "", "project name (default: current directory name)")
	if err := fs.Parse(args); err != nil {
		return parseExit(err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fail(stderr, *jsonOut, ExitError, "cwd", err.Error())
	}
	if *name == "" {
		*name = filepath.Base(cwd)
	}

	path := filepath.Join(cwd, config.Filename)
	if _, err := os.Stat(path); err == nil && !*force {
		return fail(stderr, *jsonOut, ExitError, "exists", "prx.toml already exists (use --force to overwrite)")
	}

	domain := domainLabel(*name) + ".localhost"
	content := fmt.Sprintf("[project]\nname = %q\n\n[services.web]\ndomain = %q\n", *name, domain)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec // project config, not a secret
		return fail(stderr, *jsonOut, ExitError, "write", err.Error())
	}

	if *jsonOut {
		return writeJSON(stdout, map[string]any{"path": path, "project": *name, "created": true})
	}
	if richOut(stdout, false) {
		fmt.Fprintf(stdout, "%s created %s\n", ui.Tint(ui.Success, "✓"), config.Filename)
	} else {
		fmt.Fprintf(stdout, "created %s\n", config.Filename)
	}
	fmt.Fprintf(stdout, "  project: %s\n  service: web -> %s\n", *name, domain)
	fmt.Fprintln(stderr, "next: run `prx up`")
	return ExitOK
}

// domainLabel turns an arbitrary project name into a safe DNS label
// (lowercase, [a-z0-9-], no leading/trailing dash).
func domainLabel(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		case !prevDash:
			b.WriteByte('-')
			prevDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "app"
	}
	return out
}
