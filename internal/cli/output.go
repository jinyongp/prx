// Package cli implements gate's command surface and its text/json output
// contract: data on stdout, diagnostics on stderr, a single JSON value under
// --json, and stable exit codes.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"gate/internal/paths"
	"gate/internal/registry"
	"gate/internal/ui"
)

// Exit codes (see README §13).
const (
	ExitOK       = 0
	ExitError    = 1
	ExitUsage    = 2
	ExitPerm     = 3
	ExitConflict = 4
)

func registryStore() *registry.Store {
	return registry.Open(filepath.Join(paths.ConfigDir(), "registry.json"))
}

// isTTY reports whether w should receive styled status output.
func isTTY(w io.Writer) bool { return ui.ColorEnabled(w) }

// richOut is the single CLI gate for styled output: JSON always stays plain,
// otherwise the shared colour policy decides.
func richOut(w io.Writer, jsonOut bool) bool { return !jsonOut && ui.ColorEnabled(w) }

type activityHandle interface {
	Stop()
	Complete()
}

var startActivityFunc = func(stderr io.Writer, jsonOut bool, label string) activityHandle {
	return ui.StartActivity(stderr, label, ui.ActivityOptions{
		Enabled: ui.ActivityEnabled(stderr, jsonOut),
	})
}

func startActivity(stderr io.Writer, jsonOut bool, label string) activityHandle {
	return startActivityFunc(stderr, jsonOut, label)
}

func writeJSON(w io.Writer, v any) int {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return ExitError
	}
	return ExitOK
}

type errBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errEnvelope struct {
	Error errBody `json:"error"`
}

// fail reports an error: a JSON envelope on stderr under --json, else a plain
// line. It returns code so callers can `return fail(...)`.
func fail(stderr io.Writer, jsonOut bool, code int, errCode, msg string) int {
	switch {
	case jsonOut:
		enc := json.NewEncoder(stderr)
		_ = enc.Encode(errEnvelope{Error: errBody{Code: errCode, Message: msg}})
	case richOut(stderr, false):
		fmt.Fprintf(stderr, "%s %s\n", ui.Tint(ui.Danger, "gate:"), msg)
	default:
		fmt.Fprintf(stderr, "gate: %s\n", msg)
	}
	return code
}

func printSuccess(stdout io.Writer, msg string) {
	if richOut(stdout, false) {
		fmt.Fprintf(stdout, "%s %s\n", ui.Tint(ui.Success, "✓"), msg)
		return
	}
	fmt.Fprintln(stdout, msg)
}

func printOK(stdout io.Writer, msg string) {
	if richOut(stdout, false) {
		fmt.Fprintf(stdout, "%s %s\n", ui.Tint(ui.Success, "ok:"), msg)
		return
	}
	fmt.Fprintln(stdout, msg)
}

func printInfo(stdout io.Writer, msg string) {
	if richOut(stdout, false) {
		fmt.Fprintln(stdout, ui.Dim.Render(msg))
		return
	}
	fmt.Fprintln(stdout, msg)
}

func printEmpty(stdout io.Writer, richMsg, plainMsg string) {
	if richOut(stdout, false) {
		fmt.Fprintln(stdout, ui.Dim.Render(richMsg))
		return
	}
	fmt.Fprintln(stdout, plainMsg)
}

func printWarning(stderr io.Writer, msg string) {
	if richOut(stderr, false) {
		fmt.Fprintf(stderr, "%s %s\n", ui.Tint(ui.Warn, "!"), msg)
		return
	}
	fmt.Fprintf(stderr, "warning: %s\n", msg)
}

func printError(stderr io.Writer, msg string) {
	if richOut(stderr, false) {
		fmt.Fprintln(stderr, ui.Tint(ui.Danger, "error:")+" "+msg)
		return
	}
	fmt.Fprintf(stderr, "error: %s\n", msg)
}

func printKV(stdout io.Writer, label, value string) {
	if richOut(stdout, false) {
		fmt.Fprintf(stdout, "  %s  %s\n", ui.Dim.Render(label), value)
		return
	}
	fmt.Fprintf(stdout, "  %s: %s\n", label, value)
}

func statusDot(status string, color bool) string {
	if !color {
		if status == "live" {
			return "* live"
		}
		return "o down"
	}
	if status == "live" {
		return ui.Tint(ui.Success, "●") + " live"
	}
	return ui.Tint(ui.Muted, "○") + " down"
}

func printCancelled(stdout io.Writer, action string) {
	msg := strings.TrimSpace(action) + " cancelled"
	if richOut(stdout, false) {
		fmt.Fprintf(stdout, "\n%s %s\n", ui.Tint(ui.Danger, "✗"), msg)
		return
	}
	fmt.Fprintf(stdout, "\n✗ %s\n", msg)
}
