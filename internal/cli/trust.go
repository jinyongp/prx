package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sync"

	"gate/internal/ca"
	"gate/internal/expose"
	"gate/internal/paths"
	"gate/internal/proxy"
)

var (
	trustAuthorityFunc   = func(authority *ca.CA) error { return authority.Trust() }
	untrustAuthorityFunc = func(authority *ca.CA) error { return authority.Untrust() }
	exposeProviderFor    = expose.For
	exposeSessionMu      sync.Mutex
	exposeSessionRoutes  = map[string]map[string]exposeSessionRoute{}
)

type exposeSessionRoute struct {
	Auth string
}

// Trust installs the root CA into the OS and browser trust stores.
func Trust(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("trust", flag.ContinueOnError)
	if handled, code := parseFlags(fs, "trust", args, stdout, stderr); handled {
		return code
	}
	activity := startActivity(stderr, false, "preparing trust store")
	authority, err := ca.Load(paths.DataDir())
	activity.Stop()
	if err != nil {
		return fail(stderr, false, ExitError, "ca", err.Error())
	}
	if err := trustAuthorityFunc(authority); err != nil {
		if os.IsPermission(err) {
			return fail(stderr, false, ExitPerm, "permission", err.Error())
		}
		return fail(stderr, false, ExitError, "trust", err.Error())
	}
	fmt.Fprintf(stdout, "root CA trusted\nfingerprint %s\n", authority.Fingerprint())
	return ExitOK
}

// Untrust removes the root CA from the OS and browser trust stores.
func Untrust(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("untrust", flag.ContinueOnError)
	if handled, code := parseFlags(fs, "untrust", args, stdout, stderr); handled {
		return code
	}
	activity := startActivity(stderr, false, "preparing trust store")
	authority, err := ca.LoadCertificate(paths.DataDir())
	activity.Stop()
	if errors.Is(err, ca.ErrNotFound) {
		fmt.Fprintln(stdout, "root CA not found; nothing to untrust")
		return ExitOK
	}
	if err != nil {
		return fail(stderr, false, ExitError, "ca", err.Error())
	}
	if err := untrustAuthorityFunc(authority); err != nil {
		if os.IsPermission(err) {
			return fail(stderr, false, ExitPerm, "permission", err.Error())
		}
		return fail(stderr, false, ExitError, "untrust", err.Error())
	}
	fmt.Fprintf(stdout, "root CA untrusted\nfingerprint %s\n", authority.Fingerprint())
	return ExitOK
}

// Ca dispatches `gate ca export`.
func Ca(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		sp := specFor("ca")
		WriteHelp(stdout, "ca", sp.Args, sp.Summary, nil)
		return ExitOK
	}
	if len(args) == 0 || args[0] != "export" {
		usageLine(stderr, "ca")
		return ExitUsage
	}
	fs := flag.NewFlagSet("ca export", flag.ContinueOnError)
	out := fs.String("out", "gate-root.crt", "output path")
	if handled, code := parseFlags(fs, "ca export", args[1:], stdout, stderr); handled {
		return code
	}
	authority, err := ca.Load(paths.DataDir())
	if err != nil {
		return fail(stderr, false, ExitError, "ca", err.Error())
	}
	fp, err := authority.Export(*out)
	if err != nil {
		return fail(stderr, false, ExitError, "export", err.Error())
	}
	fmt.Fprintf(stdout, "exported %s\nsha256 %s\n", *out, fp)
	return ExitOK
}

// Expose publishes a service beyond this machine via a provider.
func Expose(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("expose", flag.ContinueOnError)
	via := fs.String("via", "local", "provider: local|lan|cloudflared|tailscale")
	auth := fs.String("auth", "", "require basic auth as user:pass")
	jsonOut := fs.Bool("json", false, "emit JSON")
	scopeFlags := defineDaemonScopeFlags(fs, false)
	if handled, code := parseFlags(fs, "expose", args, stdout, stderr); handled {
		return code
	}
	sel, err := registryScopeFromFlags(scopeFlags, false)
	if err != nil {
		return fail(stderr, *jsonOut, ExitUsage, "bad_scope", err.Error())
	}
	rest := fs.Args()
	if len(rest) != 1 {
		return usageFail(stderr, *jsonOut, "expose")
	}
	svc := rest[0]
	res, lerr := lookupScopedReservation(svc, sel)
	if lerr != nil {
		return fail(stderr, *jsonOut, lerr.Exit, lerr.Code, lerr.Message)
	}
	if !res.Active {
		return fail(stderr, *jsonOut, ExitError, "not_active", fmt.Sprintf("reservation %q is not active; run gate up first", svc))
	}
	provider, err := exposeProviderFor(*via)
	if err != nil {
		return fail(stderr, *jsonOut, ExitUsage, "bad_provider", err.Error())
	}
	if *auth == "" && *via != "local" {
		fmt.Fprintln(stderr, "warning: exposing without --auth; anyone with the URL can reach your dev server")
	}
	var activity activityHandle
	if exposeActivityAllowed(*via) {
		activity = startActivity(stderr, *jsonOut, "starting tunnel")
	}
	url, err := provider.Expose(context.Background(), res.Domain, expose.Opts{Auth: *auth})
	if activity != nil {
		activity.Stop()
	}
	if err != nil {
		return fail(stderr, *jsonOut, ExitError, "expose_failed", err.Error())
	}

	// Mark the route exposed (so non-loopback clients are allowed) and apply
	// optional auth, then hot-reload the daemon. Auth is session-scoped: it
	// lives in the in-memory route table, not the persisted registry.
	scope := sel.Scope
	client := daemonClientFor(scope)
	if client.IsRunning() {
		if reg, rerr := registryStore().Read(); rerr == nil {
			routes := activeRoutesForScope(reg, scope)
			applyExposeSession(scope, routes, res.Domain, *auth)
			if serr := setDaemonRoutesWithActivity(scope, routes, stderr, *jsonOut, "reloading routes"); serr != nil {
				return fail(stderr, *jsonOut, ExitError, "reload_failed", serr.Error())
			}
		}
	}

	if *jsonOut {
		out := map[string]any{"service": svc, "provider": *via, "public_url": url, "target": res.Domain}
		if res.Project != "" {
			out["project"] = res.Project
		} else {
			out["global"] = true
		}
		return writeJSON(stdout, out)
	}
	fmt.Fprintf(stdout, "%s exposed via %s\n  %s -> %s\n", displayReservationOwner(res), *via, url, res.Domain)
	return ExitOK
}

func exposeActivityAllowed(via string) bool {
	return via == expose.ProviderCloudflared || via == expose.ProviderTailscale
}

func applyExposeSession(scope daemonScope, routes []proxy.Route, domain, auth string) {
	exposeSessionMu.Lock()
	defer exposeSessionMu.Unlock()
	key := scope.String()
	if exposeSessionRoutes[key] == nil {
		exposeSessionRoutes[key] = map[string]exposeSessionRoute{}
	}
	exposeSessionRoutes[key][domain] = exposeSessionRoute{Auth: auth}
	for i := range routes {
		session, ok := exposeSessionRoutes[key][routes[i].Domain]
		if !ok {
			continue
		}
		routes[i].Exposed = true
		routes[i].Auth = session.Auth
	}
}
