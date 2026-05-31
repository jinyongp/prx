package expose

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sync"
)

// Cloudflared exposes a route as a public URL via the cloudflared binary.
type Cloudflared struct {
	mu  sync.Mutex
	cmd *exec.Cmd
}

var trycloudflareRe = regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)

// Expose starts a quick tunnel to the local HTTPS address for domain and
// returns the public URL cloudflared prints.
func (c *Cloudflared) Expose(ctx context.Context, domain string, _ Opts) (string, error) {
	//nolint:gosec // G204: fixed binary; domain comes from the project config.
	cmd := exec.CommandContext(ctx, "cloudflared", "tunnel", "--url", "https://"+domain)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("expose: cloudflared not available: %w", err)
	}
	c.mu.Lock()
	c.cmd = cmd
	c.mu.Unlock()

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		if url := trycloudflareRe.FindString(scanner.Text()); url != "" {
			return url, nil
		}
	}
	return "", fmt.Errorf("expose: cloudflared did not report a public URL")
}

// Close terminates the tunnel.
func (c *Cloudflared) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Kill()
	}
	return nil
}

// Tailscale exposes a route over a tailnet via `tailscale serve`.
type Tailscale struct{}

// Expose publishes the local HTTPS address through tailscale serve.
func (Tailscale) Expose(ctx context.Context, domain string, _ Opts) (string, error) {
	//nolint:gosec // G204: fixed binary; domain comes from the project config.
	cmd := exec.CommandContext(ctx, "tailscale", "serve", "--bg", "https://"+domain)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("expose: tailscale serve failed: %w: %s", err, out)
	}
	return "https://" + domain, nil
}

// Close is a no-op; `tailscale serve reset` tears down manually.
func (Tailscale) Close() error { return nil }
