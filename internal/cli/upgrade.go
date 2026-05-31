package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	upgradeScriptURL = "https://raw.githubusercontent.com/jinyongp/prx/main/scripts/install.sh"
	githubLatestAPI  = "https://api.github.com/repos/jinyongp/prx/releases/latest"
	defaultUserAgent = "prx-upgrade"
)

var currentVersion = "dev"

// SetVersion stores the currently running prx version for upgrade decisions.
func SetVersion(v string) {
	currentVersion = v
}

// Upgrade downloads and executes the upstream install script to replace the current
// prx binary with the latest release.
func Upgrade(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("upgrade", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return parseExit(err)
	}
	if fs.NArg() != 0 {
		return fail(stderr, false, ExitUsage, "usage", "usage: prx upgrade")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	latestTag, err := latestReleaseTag(ctx)
	if err == nil {
		if current := normalizedVersion(currentVersion); current != "" && current != "dev" {
			if latest := normalizedVersion(latestTag); latest == current {
				fmt.Fprintf(stdout, "prx is already up to date (%s)\n", latestTag)
				return ExitOK
			}
		}
	} else {
		_, _ = fmt.Fprintf(stderr, "warning: unable to check latest version: %v\n", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upgradeScriptURL, nil)
	if err != nil {
		return fail(stderr, false, ExitError, "upgrade", err.Error())
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fail(stderr, false, ExitError, "upgrade", "failed to download install script: "+err.Error())
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode != http.StatusOK {
		return fail(stderr, false, ExitError, "upgrade", fmt.Sprintf("failed to download install script: %s", res.Status))
	}

	script, err := os.CreateTemp("", "prx-upgrade-*.sh")
	if err != nil {
		return fail(stderr, false, ExitError, "upgrade", err.Error())
	}
	defer func() {
		_ = os.Remove(script.Name())
	}()

	if _, err := io.Copy(script, res.Body); err != nil {
		return fail(stderr, false, ExitError, "upgrade", err.Error())
	}
	if err := script.Chmod(0o755); err != nil {
		return fail(stderr, false, ExitError, "upgrade", err.Error())
	}
	if err := script.Close(); err != nil {
		return fail(stderr, false, ExitError, "upgrade", err.Error())
	}

	//nolint:gosec // G204: executing trusted, repo-fixed upgrade script.
	cmd := exec.Command("sh", script.Name())
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fail(stderr, false, ExitError, "upgrade", err.Error())
	}
	fmt.Fprintln(stdout, "upgrade complete")
	return ExitOK
}

type githubRelease struct {
	TagName string `json:"tag_name"`
}

func latestReleaseTag(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubLatestAPI, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to check latest release: %s", res.Status)
	}
	var release githubRelease
	if err := json.NewDecoder(res.Body).Decode(&release); err != nil {
		return "", err
	}
	if release.TagName == "" {
		return "", fmt.Errorf("latest release has empty tag_name")
	}
	return release.TagName, nil
}

func normalizedVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(strings.ToLower(v), "v")
	return v
}
