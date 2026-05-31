package expose

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestForSelectsProvider(t *testing.T) {
	cases := map[string]any{
		"":            Local{},
		"local":       Local{},
		"lan":         LAN{},
		"cloudflared": &Cloudflared{},
		"tailscale":   Tailscale{},
	}
	for name := range cases {
		if _, err := For(name); err != nil {
			t.Errorf("For(%q): %v", name, err)
		}
	}
	if _, err := For("bogus"); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestLocalExpose(t *testing.T) {
	url, err := Local{}.Expose(context.Background(), "app.localhost", Opts{})
	if err != nil || url != "https://app.localhost" {
		t.Fatalf("Local.Expose = %q, %v", url, err)
	}
}

func TestLANRequiresDotLocal(t *testing.T) {
	if _, err := (LAN{}).Expose(context.Background(), "app.example.com", Opts{}); err == nil {
		t.Fatal("lan should reject non-.local domain")
	}
	url, err := LAN{}.Expose(context.Background(), "app.local", Opts{})
	if err != nil || url != "https://app.local" {
		t.Fatalf("LAN.Expose = %q, %v", url, err)
	}
}

func TestBasicAuth(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := BasicAuth(ok, "user:secret")
	srv := httptest.NewServer(h)
	defer srv.Close()

	// No credentials -> 401.
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no-creds status = %d, want 401", resp.StatusCode)
	}

	// Correct credentials -> 200.
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.SetBasicAuth("user", "secret")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("good-creds status = %d, want 200", resp.StatusCode)
	}
}

func TestBasicAuthEmptyPassthrough(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	if got := BasicAuth(ok, ""); got == nil {
		t.Fatal("empty auth should return the handler unchanged")
	}
}
