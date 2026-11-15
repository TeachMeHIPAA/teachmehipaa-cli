package testkit

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/cmdutil"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/commands"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/keyring"
)

// Clock is a settable clock whose Sleep returns at once, moving time on and
// recording how long it was asked to wait.
type Clock struct {
	now    time.Time
	sleeps []time.Duration
}

func (c *Clock) Now() time.Time          { return c.now }
func (c *Clock) Advance(d time.Duration) { c.now = c.now.Add(d) }

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
}

func NewEnv(t testing.TB) *Env {
	return &Env{
		Transport: NewTransport(t),
		Keyring:   keyring.NewMemory(),
		Clock:     &Clock{now: time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)},
		ConfigDir: t.TempDir(),
		Vars:      map[string]string{},
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
	var stdout, stderr bytes.Buffer
	code := commands.Execute(context.Background(), cmdutil.Deps{
		Transport:   e.Transport,
		Keyring:     e.Keyring,
		Clock:       e.Clock,
		Stdin:       strings.NewReader(e.Stdin),
		Stdout:      &stdout,
		Stderr:      &stderr,
		LookupEnv:   e.lookup,
		StdinIsTTY:  e.StdinIsTTY,
		StdoutIsTTY: e.StdoutIsTTY,
		ConfigDir:   e.ConfigDir,
	}, args)
	return Result{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: code}
}

func (e *Env) lookup(name string) (string, bool) {
	value, ok := e.Vars[name]
	return value, ok
}
