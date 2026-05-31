package cli

import (
	"flag"
	"fmt"
	"io"
	"text/tabwriter"

	"prx/internal/ui"
)

// CmdSpec is the single source of truth for a subcommand's name, positional
// argument signature, and one-line summary. It drives both the root usage
// screen (cmd/prx) and per-command help/usage rendering here.
type CmdSpec struct {
	Name    string
	Args    string // positional signature, e.g. "<domain> <port>"; "" if none
	Summary string
}

// Specs lists every public subcommand in display order. Keep in sync with the
// dispatch table in cmd/prx/main.go.
var Specs = []CmdSpec{
	{"init", "", "scaffold a starter prx.toml in the current directory"},
	{"up", "", "bring up the current project: reserve ports, render routes, reload"},
	{"down", "", "tear down the current project's routes and free its ports"},
	{"ls", "", "list all reservations with live/down status"},
	{"port", "<service>", "print the reserved port for a service"},
	{"add", "<domain> <port>", "reserve a port for a domain in the current project"},
	{"rm", "<domain>", "remove a domain reservation from the current project"},
	{"prune", "", "remove reservations whose project config no longer exists"},
	{"run", "<service> -- <cmd> [args]", "run a child process with PORT injected from the reservation"},
	{"daemon", "start|stop|status|logs", "control the background proxy daemon (start/stop/status/restart)"},
	{"trust", "", "install the local CA into the OS and browser trust stores"},
	{"ca", "export [--out <path>]", "export the local CA certificate"},
	{"expose", "<service> --via <provider>", "publish a local service through a public tunnel provider"},
	{"completion", "<bash|zsh|fish>", "print shell completion script (bash|zsh|fish)"},
	{"upgrade", "", "upgrade prx to the latest GitHub release"},
	{"skill", "path|print", "locate or print the bundled agent skill (path|print)"},
}

func specFor(name string) (CmdSpec, bool) {
	for _, s := range Specs {
		if s.Name == name {
			return s, true
		}
	}
	return CmdSpec{}, false
}

// FlagInfo is one row of a command's FLAGS section.
type FlagInfo struct {
	Name, Usage, Default string
}

func collectFlags(fs *flag.FlagSet) []FlagInfo {
	var out []FlagInfo
	fs.VisitAll(func(f *flag.Flag) {
		out = append(out, FlagInfo{Name: f.Name, Usage: f.Usage, Default: f.DefValue})
	})
	return out
}

func section(w io.Writer, label string) string {
	if ui.Enabled(w) {
		return ui.Section(label)
	}
	return label
}

// signature builds "prx <name> <args> [flags]".
func signature(name, args string, hasFlags bool) string {
	sig := "prx " + name
	if args != "" {
		sig += " " + args
	}
	if hasFlags {
		sig += " [flags]"
	}
	return sig
}

// usageLine prints the short "usage: ..." line shown when required arguments are
// missing. The caller is responsible for returning ExitUsage.
func usageLine(w io.Writer, name string) {
	sp, _ := specFor(name)
	fmt.Fprintf(w, "usage: %s\n", signature(name, sp.Args, false))
}

// usageFail reports a missing/invalid-argument usage error. Under --json it
// keeps the JSON error envelope contract; otherwise it prints the unified usage
// line. It returns ExitUsage so callers can `return usageFail(...)`.
func usageFail(stderr io.Writer, jsonOut bool, name string) int {
	if jsonOut {
		sp, _ := specFor(name)
		return fail(stderr, true, ExitUsage, "usage", "usage: "+signature(name, sp.Args, false))
	}
	usageLine(stderr, name)
	return ExitUsage
}

// WriteHelp renders a subcommand's detailed help in prx's unified style: a title
// line, a USAGE section, and an optional FLAGS section. Exported so cmd/prx can
// render the cobra-managed `completion` command identically.
func WriteHelp(w io.Writer, name, args, summary string, flags []FlagInfo) {
	title := "prx " + name
	switch {
	case !ui.Enabled(w) && summary != "":
		fmt.Fprintf(w, "%s — %s\n", title, summary)
	case !ui.Enabled(w):
		fmt.Fprintln(w, title)
	case summary != "":
		fmt.Fprintf(w, "%s — %s\n", ui.Tint(ui.Brand, title), ui.Dim.Render(summary))
	default:
		fmt.Fprintln(w, ui.Tint(ui.Brand, title))
	}

	fmt.Fprintf(w, "\n%s\n  %s\n", section(w, "USAGE"), signature(name, args, len(flags) > 0))

	if len(flags) > 0 {
		fmt.Fprintf(w, "\n%s\n", section(w, "FLAGS"))
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		for _, f := range flags {
			desc := f.Usage
			// Show a default only when it is meaningful — skip zero values
			// (bool false, empty string, 0) the way flag.PrintDefaults does.
			if f.Default != "" && f.Default != "false" && f.Default != "0" {
				desc = fmt.Sprintf("%s (default %q)", desc, f.Default)
			}
			fmt.Fprintf(tw, "  --%s\t%s\n", f.Name, desc)
		}
		_ = tw.Flush()
	}
}

// parseFlags is the unified flag-parsing front door for subcommands. It handles
// -h/--help by rendering detailed help to stdout (exit 0) and parse errors by
// printing the usage line to stderr (exit 2). When it returns handled=false the
// caller proceeds with fs.Args(). name need not be in Specs (sub-actions like
// "daemon status" render without a summary).
func parseFlags(fs *flag.FlagSet, name string, argv []string, stdout, stderr io.Writer) (handled bool, code int) {
	for _, a := range argv {
		if a == "--" {
			break
		}
		if a == "-h" || a == "--help" {
			sp, _ := specFor(name)
			WriteHelp(stdout, name, sp.Args, sp.Summary, collectFlags(fs))
			return true, ExitOK
		}
	}
	fs.SetOutput(stderr)
	fs.Usage = func() { usageLine(stderr, name) }
	if err := fs.Parse(argv); err != nil {
		return true, ExitUsage
	}
	return false, 0
}
