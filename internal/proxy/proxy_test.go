package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func alwaysLive(string) bool { return true }
func neverLive(string) bool  { return false }

// frontend returns an httptest server fronting s.HTTPSHandler (plaintext: TLS is
// the transport layer, not the routing logic under test).
func frontend(t *testing.T, s *Server) *httptest.Server {
	t.Helper()
	fe := httptest.NewServer(s.HTTPSHandler())
	t.Cleanup(fe.Close)
	return fe
}

func get(t *testing.T, fe *httptest.Server, host, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, fe.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = host
	resp, err := fe.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestProxiesToUpstream(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello %s", r.Host)
	}))
	defer backend.Close()

	s := New(nil, alwaysLive)
	s.SetRoutes([]Route{{Domain: "app.localhost", Upstream: backend.Listener.Addr().String()}})

	resp := get(t, frontend(t, s), "app.localhost", "/")
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "hello app.localhost") {
		t.Fatalf("body = %q (host not preserved?)", body)
	}
}

func TestUnknownHostIs404(t *testing.T) {
	s := New(nil, alwaysLive)
	resp := get(t, frontend(t, s), "ghost.localhost", "/")
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestDeadUpstreamIs502(t *testing.T) {
	s := New(nil, neverLive)
	s.SetRoutes([]Route{{Domain: "app.localhost", Upstream: "127.0.0.1:1"}})
	resp := get(t, frontend(t, s), "app.localhost", "/")
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", resp.StatusCode)
	}
	if !strings.Contains(string(body), "no dev server running") {
		t.Fatalf("missing guidance: %q", body)
	}
}

func TestHTTPRedirectsToHTTPS(t *testing.T) {
	s := New(nil, alwaysLive)
	fe := httptest.NewServer(s.HTTPHandler())
	defer fe.Close()
	fe.Client().CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	req, _ := http.NewRequest(http.MethodGet, fe.URL+"/path?q=1", nil)
	req.Host = "app.localhost"
	resp, err := fe.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want 301", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "https://app.localhost/path?q=1" {
		t.Fatalf("Location = %q", loc)
	}
}

func TestSSEStreams(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl, _ := w.(http.Flusher)
		for i := 0; i < 3; i++ {
			fmt.Fprintf(w, "data: %d\n\n", i)
			fl.Flush()
		}
	}))
	defer backend.Close()

	s := New(nil, alwaysLive)
	s.SetRoutes([]Route{{Domain: "sse.localhost", Upstream: backend.Listener.Addr().String()}})
	resp := get(t, frontend(t, s), "sse.localhost", "/")
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "data: 0") || !strings.Contains(string(body), "data: 2") {
		t.Fatalf("SSE not streamed: %q", body)
	}
}

func TestRemoteAllowed(t *testing.T) {
	cases := []struct {
		addr    string
		exposed bool
		want    bool
	}{
		{"127.0.0.1:5555", false, true},
		{"[::1]:5555", false, true},
		{"192.168.1.5:5555", false, false},
		{"192.168.1.5:5555", true, true},
	}
	for _, c := range cases {
		if got := remoteAllowed(c.addr, c.exposed); got != c.want {
			t.Errorf("remoteAllowed(%q,%v) = %v, want %v", c.addr, c.exposed, got, c.want)
		}
	}
}

func TestSetRoutesConcurrentSwap(t *testing.T) {
	s := New(nil, alwaysLive)
	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
				s.SetRoutes([]Route{{Domain: fmt.Sprintf("a%d.localhost", i%4), Upstream: "127.0.0.1:1"}})
			}
		}
	}()

	for i := 0; i < 200; i++ {
		_ = s.lookup("a1.localhost")
	}
	close(stop)
	wg.Wait()
}
