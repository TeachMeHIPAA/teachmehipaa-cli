package testkit

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/cmdutil"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/commands"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/keyring"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/version"
)

// Clock is a settable clock whose Sleep returns at once, moving time on and
// recording how long it was asked to wait.
type Clock struct {
	now    time.Time
	sleeps []time.Duration
}

// Now is the clock's current time.
func (c *Clock) Now() time.Time { return c.now }

// Advance moves the clock on by d.
func (c *Clock) Advance(d time.Duration) { c.now = c.now.Add(d) }

// Sleep records d and advances by it at once.
func (c *Clock) Sleep(_ context.Context, d time.Duration) error {
	c.sleeps = append(c.sleeps, d)
	c.Advance(d)
	return nil
}

// Sleeps returns every wait in order.
func (c *Clock) Sleeps() []time.Duration { return append([]time.Duration(nil), c.sleeps...) }

// Env is one machine: its keyring, config dir, environment variables and
// terminal. Successive Runs share it, so `login` then `learners retrieve`
// behaves like two invocations on the same machine.
type Env struct {
	Transport *Transport
	Keyring   keyring.Keyring
	Clock     *Clock
	ConfigDir string
	Vars      map[string]string
	// Stdin is what the next Run reads.
	Stdin       string
	StdinIsTTY  bool
	StdoutIsTTY bool
	StderrIsTTY bool
	// Opened is every URL handed to the browser, in order.
	Opened []string
	// NoBrowser leaves tmh without a way to open one.
	NoBrowser bool
	// BrowserErr makes opening a URL fail.
	BrowserErr error
	// Build is the tmh build under test.
	Build version.Info
	// ForwardTransport carries `--forward-to` requests, normally to an
	// httptest server.
	ForwardTransport http.RoundTripper
	// UpdateTransport answers the version check's requests to
	// api.github.com.
	UpdateTransport *Transport
}

// NewEnv is a fresh machine: logged out, with an empty config dir and a
// clock at 2026-11-15 12:00 UTC.
func NewEnv(t testing.TB) *Env {
	return &Env{
		Transport:        NewTransport(t),
		Keyring:          keyring.NewMemory(),
		Clock:            &Clock{now: time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)},
		ConfigDir:        t.TempDir(),
		Vars:             map[string]string{},
		Build:            version.Info{Version: "1.2.0", Commit: "abc1234", Date: "2026-11-01T00:00:00Z"},
		ForwardTransport: http.DefaultTransport,
		UpdateTransport:  NewTransport(t),
	}
}

// Result is what a command printed and how it exited.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Run executes `tmh <args>`.
func (e *Env) Run(args ...string) Result {
	return e.RunContext(context.Background(), args...)
}

// RunContext executes `tmh <args>` until it ends or ctx is cancelled, as
// Ctrl-C does.
func (e *Env) RunContext(ctx context.Context, args ...string) Result {
	var stdout, stderr bytes.Buffer
	code := commands.Execute(ctx, cmdutil.Deps{
		Transport:            e.Transport,
		ForwardTransport:     e.ForwardTransport,
		UpdateCheckTransport: e.UpdateTransport,
		Keyring:              e.Keyring,
		Clock:                e.Clock,
		Stdin:                strings.NewReader(e.Stdin),
		Stdout:               &stdout,
		Stderr:               &stderr,
		LookupEnv:            e.lookup,
		StdinIsTTY:           e.StdinIsTTY,
		StdoutIsTTY:          e.StdoutIsTTY,
		StderrIsTTY:          e.StderrIsTTY,
		ConfigDir:            e.ConfigDir,
		OpenBrowser:          e.openBrowser(),
		Build:                e.Build,
	}, args)
	return Result{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: code}
}

func (e *Env) openBrowser() func(string) error {
	if e.NoBrowser {
		return nil
	}
	return func(url string) error {
		e.Opened = append(e.Opened, url)
		return e.BrowserErr
	}
}

func (e *Env) lookup(name string) (string, bool) {
	value, ok := e.Vars[name]
	return value, ok
}
