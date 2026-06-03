package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strings"

	"gate/internal/config"
	"gate/internal/dns"
	"gate/internal/port"
	"gate/internal/proxy"
	"gate/internal/registry"
)

type upResult struct {
	Service   string `json:"service"`
	Domain    string `json:"domain"`
	Port      int    `json:"port"`
	Allocated bool   `json:"allocated"`
}

// Up reserves/allocates ports for the project, reflects DNS, and pushes the
// route table to a running daemon (or prints it when none is running).
func Up(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("up", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "emit JSON")
	dnsMode := fs.String("dns", "", "force DNS mode: localhost|hosts")
	startDaemon := fs.Bool("daemon", false, "start the background daemon before reloading routes")
	fs.BoolVar(startDaemon, "d", false, "start the background daemon before reloading routes")
	scopeFlags := defineDaemonScopeFlags(fs, false)
	httpsAddr := fs.String("https-addr", defaultDaemonHTTPSAddr, "daemon HTTPS listen address (with --daemon)")
	httpAddr := fs.String("http-addr", defaultDaemonHTTPAddr, "daemon HTTP listen address (with --daemon)")
	if handled, code := parseFlags(fs, "up", args, stdout, stderr); handled {
		return code
	}
	sel, err := registryScopeFromFlags(scopeFlags, false)
	if err != nil {
		return fail(stderr, *jsonOut, ExitUsage, "bad_scope", err.Error())
	}
	if *dnsMode != "" && *dnsMode != "localhost" && *dnsMode != "hosts" {
		return fail(stderr, *jsonOut, ExitUsage, "bad_dns", "dns must be localhost or hosts")
	}
	httpsAddrSet, httpAddrSet := flagSet(fs, "https-addr"), flagSet(fs, "http-addr")

	if !sel.CurrentProjectSelected {
		return upExistingScope(sel, *dnsMode, *startDaemon, *httpsAddr, *httpAddr, httpsAddrSet, httpAddrSet, stdout, stderr, *jsonOut)
	}

	project, path := sel.CurrentProject, sel.CurrentProjectPath
	scope := projectDaemonScope(project.Name)

	var results []upResult
	var routes []proxy.Route
	err = registryStore().Update(func(reg *registry.Registry) error {
		reg.Prune(configPathExists)
		used := reg.UsedPorts()
		for _, name := range sortedServices(project) {
			svc := project.Services[name]
			p, allocated, aerr := resolvePort(reg, project.Name, name, svc, used)
			if aerr != nil {
				return aerr
			}
			used[p] = true
			res := registry.Reservation{
				Project: project.Name, Service: name, Domain: svc.Domain, Port: p,
				TLS: svc.TLS, DNS: dns.ModeFor(svc.Domain, *dnsMode),
				Active: true, ConfigPath: path,
			}
			if rerr := reg.Reserve(res); rerr != nil {
				return rerr
			}
			results = append(results, upResult{Service: name, Domain: svc.Domain, Port: p, Allocated: allocated})
		}
		routes = activeRoutesForScope(reg, scope)
		return nil
	})
	var ce *registry.ConflictError
	if errors.As(err, &ce) {
		return fail(stderr, *jsonOut, ExitConflict, "conflict", ce.Error())
	}
	if err != nil {
		if errors.Is(err, port.ErrPoolExhausted) {
			return fail(stderr, *jsonOut, ExitConflict, "pool_exhausted", err.Error())
		}
		return fail(stderr, *jsonOut, ExitError, "up_failed", err.Error())
	}

	if code := ensureDNS(project, *dnsMode, stderr, *jsonOut); code != ExitOK {
		return code
	}

	reloaded, actualHTTPSAddr, code := reloadUpRoutes(scope, routes, *startDaemon, *httpsAddr, *httpAddr, httpsAddrSet, httpAddrSet, stderr, *jsonOut)
	if code != ExitOK {
		return code
	}

	if *jsonOut {
		return writeJSON(stdout, map[string]any{"project": project.Name, "reloaded": reloaded, "services": results})
	}
	for _, r := range results {
		domain := r.Domain
		if reloaded {
			domain = proxyURL(r.Domain, actualHTTPSAddr)
		}
		fmt.Fprintf(stdout, "%s/%s  %s -> :%d\n", project.Name, r.Service, domain, r.Port)
	}
	if reloaded {
		fmt.Fprintf(stdout, "proxy reloaded · %d routes active\n", len(routes))
	} else {
		fmt.Fprintln(stderr, "note: no daemon running; start it with `gate daemon start`")
	}
	return ExitOK
}

func executablePath() string {
	exe, err := os.Executable()
	if err != nil {
		return os.Args[0]
	}
	return exe
}

func configPathExists(path string) bool {
	if path == "" {
		return true
	}
	if _, err := os.Stat(path); err != nil {
		return !os.IsNotExist(err)
	}
	return true
}

func proxyURL(domain, httpsAddr string) string {
	port := proxyPort(httpsAddr)
	if port == "" {
		return "https://" + domain
	}
	return "https://" + net.JoinHostPort(domain, port)
}

func proxyPort(addr string) string {
	if addr == "" {
		return ""
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	if port == "" || port == "443" {
		return ""
	}
	return strings.TrimSpace(port)
}

func upExistingScope(sel registryScopeSelection, dnsMode string, startDaemon bool, httpsAddr, httpAddr string, httpsAddrSet, httpAddrSet bool, stdout, stderr io.Writer, jsonOut bool) int {
	scope := sel.Scope
	var results []upResult
	var activated []registry.Reservation
	var previous []projectReservation
	var beforeRoutes []proxy.Route
	var routes []proxy.Route
	err := registryStore().Update(func(reg *registry.Registry) error {
		removed := reservationsForScope(reg, sel)
		if len(removed) == 0 {
			return fmt.Errorf("no reservations for %s", scope.String())
		}
		previous = append(previous, removed...)
		beforeRoutes = activeRoutesForScope(reg, scope)
		for _, item := range removed {
			res := item.Reservation
			res.Active = true
			if dnsMode != "" {
				res.DNS = dns.ModeFor(res.Domain, dnsMode)
			} else if res.DNS == "" {
				res.DNS = dns.ModeFor(res.Domain, "")
			}
			if err := reg.Reserve(res); err != nil {
				return err
			}
			results = append(results, upResult{Service: res.Service, Domain: res.Domain, Port: res.Port})
			activated = append(activated, res)
		}
		routes = activeRoutesForScope(reg, scope)
		return nil
	})
	var ce *registry.ConflictError
	if errors.As(err, &ce) {
		return fail(stderr, jsonOut, ExitConflict, "conflict", ce.Error())
	}
	if err != nil {
		return fail(stderr, jsonOut, ExitError, "up_failed", err.Error())
	}
	var ensured []registry.Reservation
	for _, res := range activated {
		if err := ensureDomainDNS(res.Domain, res.DNS, stderr, jsonOut); err != nil {
			rollbackErr := rollbackScopedUp(previous, ensured, scope, beforeRoutes, stderr, jsonOut)
			if rollbackErr != nil {
				return fail(stderr, jsonOut, ExitError, "rollback_failed", "up failed and rollback failed: "+rollbackErr.Error())
			}
			if os.IsPermission(err) || errors.Is(err, os.ErrPermission) {
				return fail(stderr, jsonOut, ExitPerm, "permission", err.Error())
			}
			return fail(stderr, jsonOut, ExitError, "dns_failed", err.Error())
		}
		ensured = append(ensured, res)
	}
	reloaded, actualHTTPSAddr, code := reloadUpRoutes(scope, routes, startDaemon, httpsAddr, httpAddr, httpsAddrSet, httpAddrSet, stderr, jsonOut)
	if code != ExitOK {
		return code
	}
	if jsonOut {
		out := map[string]any{"reloaded": reloaded, "services": results}
		if scope.Kind == daemonScopeProject {
			out["project"] = scope.Name
		} else {
			out["global"] = true
		}
		return writeJSON(stdout, out)
	}
	for _, r := range results {
		domain := r.Domain
		if reloaded {
			domain = proxyURL(r.Domain, actualHTTPSAddr)
		}
		fmt.Fprintf(stdout, "%s/%s  %s -> :%d\n", scope.String(), r.Service, domain, r.Port)
	}
	if reloaded {
		fmt.Fprintf(stdout, "proxy reloaded · %d routes active\n", len(routes))
	} else {
		fmt.Fprintln(stderr, "note: no daemon running; start it with `gate daemon start`")
	}
	return ExitOK
}

func rollbackScopedUp(previous []projectReservation, ensured []registry.Reservation, scope daemonScope, routes []proxy.Route, stderr io.Writer, jsonOut bool) error {
	var errs []error
	if err := restoreReservations(previous, scope, routes, stderr, jsonOut); err != nil {
		errs = append(errs, err)
	}
	for i := len(ensured) - 1; i >= 0; i-- {
		res := ensured[i]
		if err := removeDomainDNS(res.Domain, res.DNS, stderr, jsonOut); err != nil {
			errs = append(errs, fmt.Errorf("remove DNS %s: %w", res.Domain, err))
		}
	}
	return errors.Join(errs...)
}

func reloadUpRoutes(scope daemonScope, routes []proxy.Route, startDaemon bool, httpsAddr, httpAddr string, httpsAddrSet, httpAddrSet bool, stderr io.Writer, jsonOut bool) (bool, string, int) {
	reloaded := false
	actualHTTPSAddr := ""
	startedPID := 0
	client := daemonClientFor(scope)
	if startDaemon {
		if st, err := client.Status(); err == nil {
			if !daemonExplicitListenMatches(st, httpsAddr, httpAddr, httpsAddrSet, httpAddrSet) {
				msg := fmt.Sprintf("daemon already running on https %s · http %s; requested https %s · http %s; run `gate daemon stop` first",
					displayListenAddr(st.HTTPSAddr), displayListenAddr(st.HTTPAddr), httpsAddr, httpAddr)
				return false, "", fail(stderr, jsonOut, ExitConflict, "daemon_start", msg)
			}
		} else {
			activity := startActivity(stderr, jsonOut, "starting daemon")
			result := startDaemonCommand(newDaemonServeCommand(executablePath(), scope.socketPath(), httpsAddr, httpAddr), client, scope)
			activity.Stop()
			if result.Code != ExitOK {
				return false, "", fail(stderr, jsonOut, result.Code, "daemon_start", result.Message)
			}
			startedPID = result.PID
		}
	}
	if client.IsRunning() {
		if code := reloadDaemonRoutes(scope, routes, stderr, jsonOut); code != ExitOK {
			if startedPID != 0 {
				cleanupStartedDaemon(client, scope, startedPID)
			}
			return false, "", code
		}
		if st, err := client.Status(); err == nil {
			actualHTTPSAddr = st.HTTPSAddr
		}
		reloaded = true
	}
	return reloaded, actualHTTPSAddr, ExitOK
}

// Down deactivates the current project's routes (reservations are preserved)
// and removes its DNS entries.
func Down(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("down", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "emit JSON")
	scopeFlags := defineDaemonScopeFlags(fs, false)
	if handled, code := parseFlags(fs, "down", args, stdout, stderr); handled {
		return code
	}
	sel, err := registryScopeFromFlags(scopeFlags, false)
	if err != nil {
		return fail(stderr, *jsonOut, ExitUsage, "bad_scope", err.Error())
	}
	if !sel.CurrentProjectSelected {
		return downExistingScope(sel, stdout, stderr, *jsonOut)
	}
	project := sel.CurrentProject
	scope := sel.Scope

	var routes []proxy.Route
	err = registryStore().Update(func(reg *registry.Registry) error {
		for name := range project.Services {
			key := registry.Key(project.Name, name)
			if res, ok := reg.Get(key); ok {
				res.Active = false
				_ = reg.Reserve(res)
			}
		}
		routes = activeRoutesForScope(reg, scope)
		return nil
	})
	if err != nil {
		return fail(stderr, *jsonOut, ExitError, "down_failed", err.Error())
	}

	for _, name := range sortedServices(project) {
		svc := project.Services[name]
		_ = removeDomainDNS(svc.Domain, "", stderr, *jsonOut)
	}

	client := daemonClientFor(scope)
	if client.IsRunning() {
		_ = setDaemonRoutesWithActivity(scope, routes, stderr, *jsonOut, "reloading routes")
	}
	if *jsonOut {
		return writeJSON(stdout, map[string]any{"project": project.Name, "down": true})
	}
	fmt.Fprintf(stdout, "%s down (reservations preserved)\n", project.Name)
	return ExitOK
}

func downExistingScope(sel registryScopeSelection, stdout, stderr io.Writer, jsonOut bool) int {
	scope := sel.Scope
	var deactivated []registry.Reservation
	var previous []projectReservation
	var beforeRoutes []proxy.Route
	var routes []proxy.Route
	err := registryStore().Update(func(reg *registry.Registry) error {
		items := reservationsForScope(reg, sel)
		if len(items) == 0 {
			return fmt.Errorf("no reservations for %s", scope.String())
		}
		previous = append(previous, items...)
		beforeRoutes = activeRoutesForScope(reg, scope)
		for _, item := range items {
			res := item.Reservation
			if res.Active {
				deactivated = append(deactivated, res)
			}
			res.Active = false
			if err := reg.Reserve(res); err != nil {
				return err
			}
		}
		routes = activeRoutesForScope(reg, scope)
		return nil
	})
	if err != nil {
		return fail(stderr, jsonOut, ExitError, "down_failed", err.Error())
	}
	for i, res := range deactivated {
		if err := removeDomainDNS(res.Domain, res.DNS, stderr, jsonOut); err != nil {
			rollbackErr := restoreProjectDNS(reservationsFromRegistry(deactivated[:i]), stderr, jsonOut)
			rollbackErr = errors.Join(rollbackErr, restoreReservations(previous, scope, beforeRoutes, stderr, jsonOut))
			if rollbackErr != nil {
				return fail(stderr, jsonOut, ExitError, "rollback_failed", "down failed and rollback failed: "+rollbackErr.Error())
			}
			if os.IsPermission(err) || errors.Is(err, os.ErrPermission) {
				return fail(stderr, jsonOut, ExitPerm, "permission", err.Error())
			}
			return fail(stderr, jsonOut, ExitError, "dns_failed", err.Error())
		}
	}
	client := daemonClientFor(scope)
	if client.IsRunning() {
		if err := setDaemonRoutesWithActivity(scope, routes, stderr, jsonOut, "reloading routes"); err != nil {
			rollbackErr := restoreProjectDNS(reservationsFromRegistry(deactivated), stderr, jsonOut)
			rollbackErr = errors.Join(rollbackErr, restoreReservations(previous, scope, beforeRoutes, stderr, jsonOut))
			if rollbackErr != nil {
				return fail(stderr, jsonOut, ExitError, "rollback_failed", "down failed and rollback failed: "+rollbackErr.Error())
			}
			return fail(stderr, jsonOut, ExitError, "reload_failed", err.Error())
		}
	}
	if jsonOut {
		out := map[string]any{"down": true}
		if scope.Kind == daemonScopeProject {
			out["project"] = scope.Name
		} else {
			out["global"] = true
		}
		return writeJSON(stdout, out)
	}
	fmt.Fprintf(stdout, "%s down (reservations preserved)\n", scope.String())
	return ExitOK
}

func reservationsFromRegistry(reservations []registry.Reservation) []projectReservation {
	out := make([]projectReservation, 0, len(reservations))
	for _, res := range reservations {
		out = append(out, projectReservation{Key: registry.Key(res.Project, res.Service), Reservation: res})
	}
	return out
}

func resolvePort(reg *registry.Registry, project, name string, svc config.Service, used map[int]bool) (int, bool, error) {
	if svc.Port != 0 {
		if existing, ok := reg.Get(registry.Key(project, name)); ok && existing.Port == svc.Port {
			return existing.Port, false, nil
		}
		return svc.Port, true, nil
	}
	if existing, ok := reg.Get(registry.Key(project, name)); ok && existing.Port != 0 {
		return existing.Port, false, nil
	}
	p, err := port.Allocate(port.DefaultPool, used)
	return p, true, err
}

func ensureDNS(project *config.Project, mode string, stderr io.Writer, jsonOut bool) int {
	for _, name := range sortedServices(project) {
		domain := project.Services[name].Domain
		if err := ensureDomainDNS(domain, mode, stderr, jsonOut); err != nil {
			if os.IsPermission(err) || errors.Is(err, os.ErrPermission) {
				return fail(stderr, jsonOut, ExitPerm, "permission", err.Error())
			}
			return fail(stderr, jsonOut, ExitError, "dns_failed", err.Error())
		}
	}
	return ExitOK
}

func activeRoutesForScope(reg *registry.Registry, scope daemonScope) []proxy.Route {
	var rs []proxy.Route
	for _, k := range reg.Keys() {
		res := reg.Services[k]
		if !res.Active || res.Port == 0 {
			continue
		}
		if scope.Kind == daemonScopeProject && res.Project != scope.Name {
			continue
		}
		if scope.Kind == daemonScopeGlobal && (res.Project != "" || !res.Standalone) {
			continue
		}
		rs = append(rs, proxy.Route{Domain: res.Domain, Upstream: fmt.Sprintf("127.0.0.1:%d", res.Port)})
	}
	return rs
}

func sortedServices(p *config.Project) []string {
	names := make([]string, 0, len(p.Services))
	for n := range p.Services {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
