package cli

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gate/internal/config"
	"gate/internal/ui"

	"golang.org/x/term"
)

// Init scaffolds a starter gate.toml in the current directory.
func Init(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "emit JSON")
	force := fs.Bool("force", false, "overwrite an existing gate.toml")
	name := fs.String("name", "", "project name (default: current directory name)")
	yes := fs.Bool("yes", false, "create a default gate.toml without prompts")
	fs.BoolVar(yes, "y", false, "create a default gate.toml without prompts")
	if handled, code := parseFlags(fs, "init", args, stdout, stderr); handled {
		return code
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
		return fail(stderr, *jsonOut, ExitError, "exists", "gate.toml already exists (use --force to overwrite)")
	}

	spec := defaultInitSpec(*name)
	if !*yes {
		if !stdinIsTerminal() {
			return fail(stderr, *jsonOut, ExitUsage, "interactive_required", "run `gate init -y` to create a default config non-interactively")
		}
		spec, err = promptInitSpec(stdout, *name)
		if err != nil {
			return fail(stderr, *jsonOut, ExitError, "init", err.Error())
		}
	}

	content := renderInitSpec(spec)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec // project config, not a secret
		return fail(stderr, *jsonOut, ExitError, "write", err.Error())
	}

	if *jsonOut {
		return writeJSON(stdout, map[string]any{"path": path, "project": spec.ProjectName, "created": true})
	}
	if richOut(stdout, false) {
		fmt.Fprintf(stdout, "%s created %s\n", ui.Tint(ui.Success, "✓"), config.Filename)
	} else {
		fmt.Fprintf(stdout, "created %s\n", config.Filename)
	}
	fmt.Fprintf(stdout, "  project: %s\n", spec.ProjectName)
	for _, svc := range spec.Services {
		if svc.Port == 0 {
			fmt.Fprintf(stdout, "  service: %s -> %s\n", svc.Name, svc.Domain)
		} else {
			fmt.Fprintf(stdout, "  service: %s -> %s (:%d)\n", svc.Name, svc.Domain, svc.Port)
		}
	}
	fmt.Fprintln(stderr, "next: run `gate up -d`")
	return ExitOK
}

type initSpec struct {
	ProjectName string
	Services    []initService
}

type initService struct {
	Name   string
	Domain string
	Port   int
}

func defaultInitSpec(projectName string) initSpec {
	label := domainLabel(projectName)
	return initSpec{
		ProjectName: projectName,
		Services: []initService{{
			Name:   "web",
			Domain: label + ".localhost",
		}},
	}
}

func promptInitSpec(stdout io.Writer, defaultName string) (initSpec, error) {
	reader := bufio.NewReader(os.Stdin)
	projectName, err := promptString(reader, stdout, "Project name", defaultName)
	if err != nil {
		return initSpec{}, err
	}
	mode, err := promptChoice(reader, stdout, "Domain mode", "localhost", []string{"localhost", "custom"})
	if err != nil {
		return initSpec{}, err
	}
	servicesRaw, err := promptString(reader, stdout, "Services (comma-separated)", "web")
	if err != nil {
		return initSpec{}, err
	}
	serviceNames := splitServiceNames(servicesRaw)
	if len(serviceNames) == 0 {
		serviceNames = []string{"web"}
	}

	baseDomain := ""
	if mode == "custom" {
		baseDomain, err = promptString(reader, stdout, "Base custom domain", "local."+domainLabel(projectName)+".test")
		if err != nil {
			return initSpec{}, err
		}
		baseDomain = canonicalPromptDomain(baseDomain)
	}

	spec := initSpec{ProjectName: projectName, Services: make([]initService, 0, len(serviceNames))}
	projectLabel := domainLabel(projectName)
	for _, name := range serviceNames {
		defaultDomain := defaultServiceDomain(name, projectLabel, mode, baseDomain)
		domain, err := promptString(reader, stdout, fmt.Sprintf("Domain for %s", name), defaultDomain)
		if err != nil {
			return initSpec{}, err
		}
		portRaw, err := promptString(reader, stdout, fmt.Sprintf("Fixed port for %s (blank = auto)", name), "")
		if err != nil {
			return initSpec{}, err
		}
		port, err := parseOptionalPort(portRaw)
		if err != nil {
			return initSpec{}, err
		}
		spec.Services = append(spec.Services, initService{Name: name, Domain: canonicalPromptDomain(domain), Port: port})
	}
	return spec, nil
}

func promptString(reader *bufio.Reader, stdout io.Writer, label, def string) (string, error) {
	if def == "" {
		fmt.Fprintf(stdout, "%s: ", label)
	} else {
		fmt.Fprintf(stdout, "%s [%s]: ", label, def)
	}
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	value := strings.TrimSpace(line)
	if value == "" {
		value = def
	}
	return value, nil
}

func promptChoice(reader *bufio.Reader, stdout io.Writer, label, def string, allowed []string) (string, error) {
	for {
		value, err := promptString(reader, stdout, label, def)
		if err != nil {
			return "", err
		}
		value = strings.ToLower(strings.TrimSpace(value))
		for _, item := range allowed {
			if value == item {
				return value, nil
			}
		}
		fmt.Fprintf(stdout, "Choose one of: %s\n", strings.Join(allowed, ", "))
	}
}

func splitServiceNames(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		name := domainLabel(strings.TrimSpace(part))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

func defaultServiceDomain(serviceName, projectLabel, mode, baseDomain string) string {
	if mode == "custom" {
		if serviceName == "web" {
			return baseDomain
		}
		return serviceName + "." + baseDomain
	}
	if serviceName == "web" {
		return projectLabel + ".localhost"
	}
	return serviceName + "." + projectLabel + ".localhost"
}

func parseOptionalPort(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid port %q", raw)
	}
	return port, nil
}

func canonicalPromptDomain(domain string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
}

func renderInitSpec(spec initSpec) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[project]\nname = %q\n", spec.ProjectName)
	for _, svc := range spec.Services {
		fmt.Fprintf(&b, "\n[services.%s]\ndomain = %q\n", svc.Name, svc.Domain)
		if svc.Port != 0 {
			fmt.Fprintf(&b, "port = %d\n", svc.Port)
		}
	}
	return b.String()
}

func stdinIsTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
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
