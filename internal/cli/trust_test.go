package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gate/internal/ca"
	"gate/internal/daemon"
	"gate/internal/expose"
	"gate/internal/paths"
	"gate/internal/proxy"
	"gate/internal/registry"
)

type fakeExposeProvider struct {
	called *int
}

func (p fakeExposeProvider) Expose(_ context.Context, domain string, _ expose.Opts) (string, error) {
	if p.called != nil {
		*p.called++
	}
	return "https://" + domain, nil
}

func (fakeExposeProvider) Close() error { return nil }

func TestUntrustDoesNotGenerateMissingCA(t *testing.T) {
	isolate(t)
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	oldUntrust := untrustAuthorityFunc
	t.Cleanup(func() { untrustAuthorityFunc = oldUntrust })
	untrustAuthorityFunc = func(*ca.CA) error {
		t.Fatal("untrust should not be called without an existing CA")
		return nil
	}

	var out, errb bytes.Buffer
	if code := Untrust(nil, &out, &errb); code != ExitOK {
		t.Fatalf("Untrust exit = %d, stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "nothing to untrust") {
		t.Fatalf("stdout = %q", out.String())
	}
	if _, err := os.Stat(filepath.Join(dataHome, "gate", "ca", "root.crt")); !os.IsNotExist(err) {
		t.Fatalf("Untrust generated CA or stat failed: %v", err)
	}
}

func TestTrustStopsActivityBeforeTrustStoreCall(t *testing.T) {
	isolate(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	events := recordActivities(t)
	oldTrust := trustAuthorityFunc
	t.Cleanup(func() { trustAuthorityFunc = oldTrust })
	trustAuthorityFunc = func(*ca.CA) error {
		if got := lastEvent(*events); got != "stop:preparing trust store" {
			t.Fatalf("trust store called before activity stopped; events=%v", *events)
		}
		return nil
	}

	var out, errb bytes.Buffer
	if code := Trust(nil, &out, &errb); code != ExitOK {
		t.Fatalf("Trust exit = %d, stderr=%s", code, errb.String())
	}
}

func TestUntrustStopsActivityBeforeTrustStoreCall(t *testing.T) {
	isolate(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if _, err := ca.Load(paths.DataDir()); err != nil {
		t.Fatal(err)
	}
	events := recordActivities(t)
	oldUntrust := untrustAuthorityFunc
	t.Cleanup(func() { untrustAuthorityFunc = oldUntrust })
	untrustAuthorityFunc = func(*ca.CA) error {
		if got := lastEvent(*events); got != "stop:preparing trust store" {
			t.Fatalf("trust store called before activity stopped; events=%v", *events)
		}
		return nil
	}

	var out, errb bytes.Buffer
	if code := Untrust(nil, &out, &errb); code != ExitOK {
		t.Fatalf("Untrust exit = %d, stderr=%s", code, errb.String())
	}
}

func TestUntrustRemovesExistingCA(t *testing.T) {
	isolate(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	authority, err := ca.Load(paths.DataDir())
	if err != nil {
		t.Fatal(err)
	}
	oldUntrust := untrustAuthorityFunc
	t.Cleanup(func() { untrustAuthorityFunc = oldUntrust })
	var fingerprint string
	untrustAuthorityFunc = func(next *ca.CA) error {
		fingerprint = next.Fingerprint()
		return nil
	}

	var out, errb bytes.Buffer
	if code := Untrust(nil, &out, &errb); code != ExitOK {
		t.Fatalf("Untrust exit = %d, stderr=%s", code, errb.String())
	}
	if fingerprint != authority.Fingerprint() {
		t.Fatalf("untrusted fingerprint = %q, want %q", fingerprint, authority.Fingerprint())
	}
	if !strings.Contains(out.String(), "root CA untrusted") || !strings.Contains(out.String(), authority.Fingerprint()) {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestUntrustWorksWithoutRootKey(t *testing.T) {
	isolate(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	authority, err := ca.Load(paths.DataDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(paths.DataDir(), "ca", "root.key")); err != nil {
		t.Fatal(err)
	}
	oldUntrust := untrustAuthorityFunc
	t.Cleanup(func() { untrustAuthorityFunc = oldUntrust })
	var fingerprint string
	untrustAuthorityFunc = func(next *ca.CA) error {
		fingerprint = next.Fingerprint()
		return nil
	}

	var out, errb bytes.Buffer
	if code := Untrust(nil, &out, &errb); code != ExitOK {
		t.Fatalf("Untrust exit = %d, stderr=%s", code, errb.String())
	}
	if fingerprint != authority.Fingerprint() {
		t.Fatalf("untrusted fingerprint = %q, want %q", fingerprint, authority.Fingerprint())
	}
}

func TestUntrustPermissionError(t *testing.T) {
	isolate(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if _, err := ca.Load(paths.DataDir()); err != nil {
		t.Fatal(err)
	}
	oldUntrust := untrustAuthorityFunc
	t.Cleanup(func() { untrustAuthorityFunc = oldUntrust })
	untrustAuthorityFunc = func(*ca.CA) error {
		return os.ErrPermission
	}

	var out, errb bytes.Buffer
	if code := Untrust(nil, &out, &errb); code != ExitPerm {
		t.Fatalf("Untrust exit = %d, want permission; stdout=%s stderr=%s", code, out.String(), errb.String())
	}
}

func TestUntrustGenericError(t *testing.T) {
	isolate(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if _, err := ca.Load(paths.DataDir()); err != nil {
		t.Fatal(err)
	}
	oldUntrust := untrustAuthorityFunc
	t.Cleanup(func() { untrustAuthorityFunc = oldUntrust })
	untrustAuthorityFunc = func(*ca.CA) error {
		return errors.New("trust store failed")
	}

	var out, errb bytes.Buffer
	if code := Untrust(nil, &out, &errb); code != ExitError {
		t.Fatalf("Untrust exit = %d, want error; stdout=%s stderr=%s", code, out.String(), errb.String())
	}
}

func TestExposeScopedGlobalAndNamedProjectReload(t *testing.T) {
	isolate(t)
	shortConfigDir, err := os.MkdirTemp("/tmp", "gate-cli-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(shortConfigDir) })
	t.Setenv("XDG_CONFIG_HOME", shortConfigDir)
	t.Setenv("XDG_STATE_HOME", shortConfigDir)
	if err := registryStore().Update(func(r *registry.Registry) error {
		if err := r.Reserve(registry.Reservation{Service: "web", Domain: "web.localhost", Port: 4400, Standalone: true, Active: true}); err != nil {
			return err
		}
		return r.Reserve(registry.Reservation{Project: "demo", Service: "api", Domain: "api.localhost", Port: 4401, Active: true})
	}); err != nil {
		t.Fatal(err)
	}

	globalSrv := proxy.New(nil, nil)
	stopGlobal, err := daemon.ServeAdmin(context.Background(), globalDaemonScope().socketPath(), globalSrv)
	if err != nil {
		t.Fatalf("ServeAdmin global: %v", err)
	}
	defer stopGlobal()
	projectSrv := proxy.New(nil, nil)
	stopProject, err := daemon.ServeAdmin(context.Background(), projectDaemonScope("demo").socketPath(), projectSrv)
	if err != nil {
		t.Fatalf("ServeAdmin project: %v", err)
	}
	defer stopProject()

	oldSetRoutes := setDaemonRoutesFunc
	t.Cleanup(func() { setDaemonRoutesFunc = oldSetRoutes })
	var calls []struct {
		scope  string
		routes []proxy.Route
	}
	setDaemonRoutesFunc = func(scope daemonScope, routes []proxy.Route) error {
		calls = append(calls, struct {
			scope  string
			routes []proxy.Route
		}{scope: scope.String(), routes: append([]proxy.Route{}, routes...)})
		return oldSetRoutes(scope, routes)
	}

	var out, errb bytes.Buffer
	if code := Expose([]string{"-g", "web", "--via", "local"}, &out, &errb); code != ExitOK {
		t.Fatalf("Expose -g exit = %d, stderr=%s", code, errb.String())
	}
	if code := Expose([]string{"-p", "demo", "api", "--via", "local", "--auth", "user:pass"}, &out, &errb); code != ExitOK {
		t.Fatalf("Expose -p exit = %d, stderr=%s", code, errb.String())
	}
	if len(calls) != 2 {
		t.Fatalf("reload calls = %+v", calls)
	}
	if calls[0].scope != "global" || len(calls[0].routes) != 1 || calls[0].routes[0].Domain != "web.localhost" || !calls[0].routes[0].Exposed {
		t.Fatalf("global reload = %+v", calls[0])
	}
	if calls[1].scope != "project:demo" || len(calls[1].routes) != 1 || calls[1].routes[0].Domain != "api.localhost" || !calls[1].routes[0].Exposed || calls[1].routes[0].Auth != "user:pass" {
		t.Fatalf("project reload = %+v", calls[1])
	}
}

func TestExposeRejectsInactiveReservationBeforeProviderCall(t *testing.T) {
	isolate(t)
	if err := registryStore().Update(func(r *registry.Registry) error {
		return r.Reserve(registry.Reservation{Service: "web", Domain: "web.localhost", Port: 4400, Standalone: true})
	}); err != nil {
		t.Fatal(err)
	}
	oldProvider := exposeProviderFor
	t.Cleanup(func() { exposeProviderFor = oldProvider })
	calls := 0
	exposeProviderFor = func(string) (expose.Provider, error) {
		return fakeExposeProvider{called: &calls}, nil
	}

	var out, errb bytes.Buffer
	if code := Expose([]string{"-g", "web", "--via", "local"}, &out, &errb); code != ExitError {
		t.Fatalf("Expose inactive exit = %d, stderr=%s", code, errb.String())
	}
	if calls != 0 {
		t.Fatalf("provider called %d times", calls)
	}
}

func TestExposePreservesExistingSessionRoutesInScope(t *testing.T) {
	isolate(t)
	shortConfigDir, err := os.MkdirTemp("/tmp", "gate-cli-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(shortConfigDir) })
	t.Setenv("XDG_CONFIG_HOME", shortConfigDir)
	t.Setenv("XDG_STATE_HOME", shortConfigDir)
	if err := registryStore().Update(func(r *registry.Registry) error {
		if err := r.Reserve(registry.Reservation{Service: "web", Domain: "web.localhost", Port: 4400, Standalone: true, Active: true}); err != nil {
			return err
		}
		return r.Reserve(registry.Reservation{Service: "api", Domain: "api.localhost", Port: 4401, Standalone: true, Active: true})
	}); err != nil {
		t.Fatal(err)
	}
	globalSrv := proxy.New(nil, nil)
	stopGlobal, err := daemon.ServeAdmin(context.Background(), globalDaemonScope().socketPath(), globalSrv)
	if err != nil {
		t.Fatalf("ServeAdmin global: %v", err)
	}
	defer stopGlobal()
	oldSetRoutes := setDaemonRoutesFunc
	t.Cleanup(func() { setDaemonRoutesFunc = oldSetRoutes })
	var calls [][]proxy.Route
	setDaemonRoutesFunc = func(scope daemonScope, routes []proxy.Route) error {
		if scope.String() != "global" {
			t.Fatalf("scope = %s", scope.String())
		}
		calls = append(calls, append([]proxy.Route{}, routes...))
		return oldSetRoutes(scope, routes)
	}

	var out, errb bytes.Buffer
	if code := Expose([]string{"-g", "web", "--via", "local", "--auth", "web:pass"}, &out, &errb); code != ExitOK {
		t.Fatalf("Expose web exit = %d, stderr=%s", code, errb.String())
	}
	if code := Expose([]string{"-g", "api", "--via", "local", "--auth", "api:pass"}, &out, &errb); code != ExitOK {
		t.Fatalf("Expose api exit = %d, stderr=%s", code, errb.String())
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %+v", calls)
	}
	final := calls[1]
	var sawWeb, sawAPI bool
	for _, route := range final {
		if route.Domain == "web.localhost" && route.Exposed && route.Auth == "web:pass" {
			sawWeb = true
		}
		if route.Domain == "api.localhost" && route.Exposed && route.Auth == "api:pass" {
			sawAPI = true
		}
	}
	if !sawWeb || !sawAPI {
		t.Fatalf("final routes = %+v", final)
	}
}
