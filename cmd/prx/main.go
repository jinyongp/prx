// Command prx is a local-development global HTTPS reverse proxy and port
// registry, shipped as a single Go binary.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"prx/internal/cli"
	"prx/internal/ui"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	cli.SetVersion(version)
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// command implements a single prx subcommand and returns a process exit code.
type command func(args []string, stdout, stderr io.Writer) int

// commands is the subcommand dispatch table. Subcommands register here as
// features land across the implementation phases.
var commands = map[string]command{
	"init":    cli.Init,
	"up":      cli.Up,
	"down":    cli.Down,
	"ls":      cli.Ls,
	"port":    cli.Port,
	"add":     cli.Add,
	"rm":      cli.Rm,
	"prune":   cli.Prune,
	"run":     cli.Run,
	"daemon":  cli.Daemon,
	"trust":   cli.Trust,
	"ca":      cli.Ca,
	"expose":  cli.Expose,
	"upgrade": cli.Upgrade,
	"skill":   cli.Skill,
	"__serve": cli.Serve,
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("prx", flag.ContinueOnError)
	fs.SetOutput(stdout)
	showVersion := fs.Bool("version", false, "print version and exit")
	fs.Usage = func() { usage(stdout) }
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, version)
		return 0
	}

	rest := fs.Args()
	if len(rest) == 0 {
		usage(stdout)
		return 0
	}
	cmd, ok := commands[rest[0]]
	if !ok {
		fmt.Fprintf(stderr, "prx: unknown command %q\n", rest[0])
		usage(stderr)
		return 2
	}
	return cmd(rest[1:], stdout, stderr)
}

// commandHelp lists public subcommands in display order with a one-line
// summary. Keep in sync with the commands dispatch table; internal commands
// (prefixed "__") are intentionally omitted.
var commandHelp = []struct{ name, summary string }{
	{"init", "scaffold a starter prx.toml in the current directory"},
	{"up", "bring up the current project: reserve ports, render routes, reload"},
	{"down", "tear down the current project's routes and free its ports"},
	{"ls", "list all reservations with live/down status"},
	{"port", "print the reserved port for a service"},
	{"add", "reserve a port for a domain in the current project"},
	{"rm", "remove a domain reservation from the current project"},
	{"prune", "remove reservations whose project config no longer exists"},
	{"run", "run a child process with PORT injected from the reservation"},
	{"daemon", "control the background proxy daemon (start/stop/status/restart)"},
	{"trust", "install the local CA into the OS and browser trust stores"},
	{"ca", "export the local CA certificate"},
	{"expose", "publish a local service through a public tunnel provider"},
	{"upgrade", "upgrade prx to the latest GitHub release"},
	{"skill", "locate or print the bundled agent skill (path|print)"},
}

func usage(w io.Writer) {
	if ui.Enabled(w) {
		usageRich(w)
		return
	}
	fmt.Fprint(w, `prx — local-dev HTTPS reverse proxy + port registry

usage:
  prx [--version] <command> [args]

commands:
`)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, c := range commandHelp {
		fmt.Fprintf(tw, "  %s\t%s\n", c.name, c.summary)
	}
	if err := tw.Flush(); err != nil {
		fmt.Fprintln(w, "prx: failed to render usage table", err)
	}
	fmt.Fprint(w, "\nRun 'prx <command> -h' for command-specific flags.\n")
}

// commandGroups arranges commands into labelled sections for the rich usage
// screen. Every commandHelp entry should appear in some group; any that does
// not is collected under "MISC" so nothing silently disappears.
var commandGroups = []struct {
	title string
	names []string
}{
	{"PROJECT", []string{"init", "up", "down", "ls", "run", "port"}},
	{"REGISTRY", []string{"add", "rm", "prune"}},
	{"DAEMON", []string{"daemon"}},
	{"TLS", []string{"trust", "ca"}},
	{"SHARE", []string{"expose"}},
	{"MAINTENANCE", []string{"upgrade", "skill"}},
}

// usageRich renders a styled, grouped usage screen for TTYs.
func usageRich(w io.Writer) {
	summary := make(map[string]string, len(commandHelp))
	width := 0
	for _, c := range commandHelp {
		summary[c.name] = c.summary
		if len(c.name) > width {
			width = len(c.name)
		}
	}

	fmt.Fprintln(w, ui.Title("prx", "local-dev HTTPS reverse proxy + port registry"))
	fmt.Fprintf(w, "\n%s\n  prx [--version] <command> [args]\n", ui.Section("USAGE"))

	grouped := map[string]bool{}
	for _, g := range commandGroups {
		fmt.Fprintf(w, "\n%s\n", ui.Section(g.title))
		for _, name := range g.names {
			grouped[name] = true
			fmt.Fprintf(w, "  %s  %s\n", ui.Command(name, width), summary[name])
		}
	}

	var misc []string
	for _, c := range commandHelp {
		if !grouped[c.name] {
			misc = append(misc, c.name)
		}
	}
	if len(misc) > 0 {
		fmt.Fprintf(w, "\n%s\n", ui.Section("MISC"))
		for _, name := range misc {
			fmt.Fprintf(w, "  %s  %s\n", ui.Command(name, width), summary[name])
		}
	}

	fmt.Fprintf(w, "\n%s\n", ui.Dim.Render("Run 'prx <command> -h' for command-specific flags."))
}
