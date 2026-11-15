package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/cmdutil"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/config"
)

const (
	latestReleaseURL    = "https://api.github.com/repos/TeachMeHIPAA/teachmehipaa-cli/releases/latest"
	releaseTagURL       = "https://github.com/TeachMeHIPAA/teachmehipaa-cli/releases/tag/"
	updateCheckInterval = 24 * time.Hour
	// updateCheckTimeout is the longest the check may hold a command up.
	updateCheckTimeout = time.Second
)

// releaseVersion is semver with an optional v. Anything else from the
// network is ignored, so a tag can't smuggle terminal escapes onto stderr.
var releaseVersion = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?(?:\+[0-9A-Za-z.-]+)?$`)

// updateNotice asks GitHub for tmh's latest release while the command runs,
// at most once a day, and prints one line on stderr when it is newer. It is
// the only request tmh sends anywhere but the API (ADR-0007), and nothing
// it does can fail the command.
type updateNotice struct {
	deps   cmdutil.Deps
	latest chan string
	ctx    context.Context
	cancel context.CancelFunc
}

func (n *updateNotice) start(cmd *cobra.Command) {
	if !n.due(cmd) {
		return
	}
	n.ctx, n.cancel = context.WithTimeout(cmd.Context(), updateCheckTimeout)
	n.latest = make(chan string, 1)
	go func() { n.latest <- fetchLatestRelease(n.ctx, n.deps.Transport) }()
}

// finish waits for the check no longer than its timeout, counted from when
// it started alongside the command.
func (n *updateNotice) finish() {
	if n.latest == nil {
		return
	}
	defer n.cancel()
	select {
	case latest := <-n.latest:
		current := n.deps.Build.Version
		if newerRelease(latest, current) {
			fmt.Fprintf(n.deps.Stderr, "A new version of tmh is available: v%s (you have v%s). %sv%[1]s\n",
				strings.TrimPrefix(latest, "v"), strings.TrimPrefix(current, "v"), releaseTagURL)
		}
	case <-n.ctx.Done():
	}
}

// due records the check before making it, so a failing or slow network
// still costs at most one attempt a day.
func (n *updateNotice) due(cmd *cobra.Command) bool {
	if n.isSet("TMH_NO_UPDATE_CHECK") || n.isSet("CI") || !n.deps.StderrIsTTY ||
		!releaseVersion.MatchString(n.deps.Build.Version) || isCompletion(cmd) {
		return false
	}
	store := config.StateStore{Dir: n.deps.ConfigDir}
	// An unreadable state file is replaced, not a reason to skip forever.
	state, _ := store.Load()
	now := n.deps.Clock.Now()
	if !state.UpdateCheckedAt.IsZero() && now.Sub(state.UpdateCheckedAt) < updateCheckInterval {
		return false
	}
	state.UpdateCheckedAt = now
	return store.Save(state) == nil
}

func (n *updateNotice) isSet(name string) bool {
	value, _ := n.deps.LookupEnv(name)
	return value != ""
}

// isCompletion is true for the scripts shells source and the completions
// they request on every Tab, where a notice would corrupt the output.
func isCompletion(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "completion" || strings.HasPrefix(c.Name(), cobra.ShellCompRequestCmd) {
			return true
		}
	}
	return false
}

// fetchLatestRelease returns the latest release's tag, or "" on any failure.
func fetchLatestRelease(ctx context.Context, transport http.RoundTripper) string {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return ""
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", cmdutil.UserAgent())
	response, err := (&http.Client{Transport: transport, Timeout: updateCheckTimeout}).Do(request)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	var release struct {
		TagName string `json:"tag_name"`
	}
	if response.StatusCode != http.StatusOK ||
		json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&release) != nil {
		return ""
	}
	return release.TagName
}

// newerRelease compares semver precedence, except that prerelease
// identifiers aren't ranked against each other: GitHub's latest release is
// never a prerelease.
func newerRelease(latest, current string) bool {
	latestCore, latestPre, ok := parseRelease(latest)
	if !ok {
		return false
	}
	currentCore, currentPre, ok := parseRelease(current)
	if !ok {
		return false
	}
	for i := range latestCore {
		if latestCore[i] != currentCore[i] {
			return latestCore[i] > currentCore[i]
		}
	}
	return currentPre != "" && latestPre == ""
}

func parseRelease(version string) (core [3]int, prerelease string, ok bool) {
	match := releaseVersion.FindStringSubmatch(version)
	if match == nil {
		return core, "", false
	}
	for i := range core {
		n, err := strconv.Atoi(match[i+1])
		if err != nil {
			return core, "", false
		}
		core[i] = n
	}
	return core, match[4], true
}
