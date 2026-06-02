// Package expose abstracts how a local route is reached from beyond the host:
// only this machine (local), the LAN (mDNS), or a public URL (cloudflared,
// tailscale). Providers are pluggable behind a common interface.
package expose

import (
	"context"
	"fmt"
	"strings"
)

// Provider names.
const (
	ProviderLocal       = "local"
	ProviderLAN         = "lan"
	ProviderCloudflared = "cloudflared"
	ProviderTailscale   = "tailscale"
)

// Opts configures an exposure.
type Opts struct {
	// Auth, if non-empty as "user:pass", requires basic auth at the proxy.
	Auth string
}

// Provider establishes external reachability for a domain and reports the URL
// to use. Close tears it down.
type Provider interface {
	Expose(ctx context.Context, domain string, opts Opts) (publicURL string, err error)
	Close() error
}

// For returns the named provider.
func For(name string) (Provider, error) {
	switch name {
	case ProviderLocal, "":
		return Local{}, nil
	case ProviderLAN:
		return LAN{}, nil
	case ProviderCloudflared:
		return &Cloudflared{}, nil
	case ProviderTailscale:
		return &Tailscale{}, nil
	default:
		return nil, fmt.Errorf("expose: unknown provider %q", name)
	}
}

// Local exposes nothing beyond this machine; the URL is the local HTTPS address.
type Local struct{}

// Expose returns the local HTTPS URL.
func (Local) Expose(_ context.Context, domain string, _ Opts) (string, error) {
	return "https://" + domain, nil
}

// Close is a no-op.
func (Local) Close() error { return nil }

// LAN advertises over mDNS, which requires a ".local" domain.
type LAN struct{}

// Expose validates the mDNS constraint and returns the LAN URL. Other devices
// must install the gate root CA (gate ca export) to trust it.
func (LAN) Expose(_ context.Context, domain string, _ Opts) (string, error) {
	if !strings.HasSuffix(domain, ".local") {
		return "", fmt.Errorf("expose: lan requires a .local domain, got %q", domain)
	}
	return "https://" + domain, nil
}

// Close is a no-op.
func (LAN) Close() error { return nil }
