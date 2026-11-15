// Package cmdutil is what every tmh command is built from: the injected
// dependencies, the selected profile, an API client and output helpers. Both
// the hand-written commands and the generated command tree depend on it.
package cmdutil

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"

	"github.com/spf13/pflag"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/api"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/clock"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/config"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/keyring"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/output"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/version"
)

// Deps is everything tmh touches outside its own process memory. main wires
// the real ones; tests pass fakes.
type Deps struct {
	Transport http.RoundTripper
	// ForwardTransport carries `--forward-to` requests to the developer's
	// own server. It is separate so those never pass through anything
	// meant for the API.
	ForwardTransport http.RoundTripper
	Keyring          keyring.Keyring
	Clock            clock.Clock
	Stdin            io.Reader
	Stdout           io.Writer
	Stderr           io.Writer
	LookupEnv        func(name string) (string, bool)
	// StdinIsTTY means a person can answer prompts.
	StdinIsTTY bool
	// StdoutIsTTY selects pretty, colored output over compact JSON.
	StdoutIsTTY bool
	// StderrIsTTY means a person sees stderr, so notices are worth printing.
	StderrIsTTY bool
	// ReadSecret reads a secret without echoing it. Nil reads a line from
	// Stdin, which is what tests and pipes need.
	ReadSecret func() (string, error)
	// ConfigDir holds config.toml, normally config.DefaultDir().
	ConfigDir string
	// OpenBrowser opens a URL in the person's browser; nil when there is no
	// way to.
	OpenBrowser func(url string) error
	// Build is this binary's version, normally version.Current().
	Build version.Info
}

// Factory is shared by every command in one invocation.
type Factory struct {
	Deps
	profileName    string
	live           bool
	maxRetries     int
	idempotencyKey string
	verbose        bool
	showHeaders    bool
	stdin          *bufio.Reader
}

func NewFactory(deps Deps) *Factory {
	return &Factory{Deps: deps, stdin: bufio.NewReader(deps.Stdin)}
}

// BindProfileFlags adds --profile and --live.
func (f *Factory) BindProfileFlags(flags *pflag.FlagSet) {
	flags.StringVar(&f.profileName, "profile", "", `profile to use: "sandbox" (default) or "live"`)
	flags.BoolVar(&f.live, "live", false, `use the "live" profile`)
}

// BindRequestFlags adds the flags every API request honors.
func (f *Factory) BindRequestFlags(flags *pflag.FlagSet) {
	flags.IntVar(&f.maxRetries, "max-retries", 2, "times to retry a transient failure of a request that is safe to replay")
	flags.StringVar(&f.idempotencyKey, "idempotency-key", "", "Idempotency-Key for the write, instead of a generated one")
	flags.BoolVar(&f.verbose, "verbose", false, "print method, path, status, latency and request id of each HTTP attempt to stderr")
	flags.BoolVar(&f.showHeaders, "show-headers", false, "print each HTTP response's status and headers to stderr")
}

// IdempotencyKey is the key for the command's write: --idempotency-key, or
// when the operation requires a key, a UUIDv4 generated once per command so
// every retry carries the same one. It is empty otherwise.
func (f *Factory) IdempotencyKey(required bool) string {
	if f.idempotencyKey == "" && required {
		f.idempotencyKey = newUUIDv4()
	}
	return f.idempotencyKey
}

// UsageError is a mistake in how tmh was invoked.
type UsageError struct{ Message string }

func (e *UsageError) Error() string { return e.Message }

// NotFoundError is something tmh looked for and the API doesn't have, found
// without a 404 from it.
type NotFoundError struct{ Message string }

func (e *NotFoundError) Error() string { return e.Message }

func (f *Factory) Profile() (config.Profile, error) {
	if f.profileName == "" {
		if f.live {
			return config.Live, nil
		}
		return config.Default, nil
	}
	profile, ok := config.LookupProfile(f.profileName)
	if !ok {
		return config.Profile{}, &UsageError{Message: fmt.Sprintf(`unknown profile %q: use "sandbox" or "live"`, f.profileName)}
	}
	if f.live && profile != config.Live {
		return config.Profile{}, &UsageError{Message: fmt.Sprintf("--live conflicts with --profile %s", profile.Name)}
	}
	return profile, nil
}

func (f *Factory) Store() config.Store {
	return config.Store{Keyring: f.Keyring, Dir: f.ConfigDir}
}

// NotLoggedInError says how to log in to the profile.
type NotLoggedInError struct {
	Profile config.Profile
	Reason  error
}

func (e *NotLoggedInError) Error() string {
	reason := fmt.Sprintf("not logged in to the %s profile", e.Profile.Name)
	if e.Reason != config.ErrNotLoggedIn {
		reason = e.Reason.Error()
	}
	login := "tmh login"
	if e.Profile == config.Live {
		login += " --live"
	}
	return fmt.Sprintf("%s; run `%s` or set TMH_CLIENT_ID and TMH_CLIENT_SECRET", reason, login)
}

func (e *NotLoggedInError) Unwrap() error { return e.Reason }

// Credentials come from TMH_CLIENT_ID and TMH_CLIENT_SECRET when set, so CI
// needs no login; otherwise from the profile's stored credentials.
func (f *Factory) Credentials(profile config.Profile) (config.Credentials, error) {
	id, hasID := f.LookupEnv("TMH_CLIENT_ID")
	secret, hasSecret := f.LookupEnv("TMH_CLIENT_SECRET")
	switch {
	case hasID && hasSecret:
		return config.Credentials{ClientID: id, ClientSecret: secret}, nil
	case hasID || hasSecret:
		return config.Credentials{}, &UsageError{Message: "set both TMH_CLIENT_ID and TMH_CLIENT_SECRET, or neither"}
	}
	credentials, err := f.Store().Load(profile.Name)
	if errors.Is(err, config.ErrNotLoggedIn) {
		return config.Credentials{}, &NotLoggedInError{Profile: profile, Reason: err}
	}
	return credentials, err
}

// APIClient builds a client for the selected profile's credentials.
func (f *Factory) APIClient() (*api.Client, config.Profile, error) {
	profile, err := f.requestProfile()
	if err != nil {
		return nil, profile, err
	}
	credentials, err := f.Credentials(profile)
	if err != nil {
		return nil, profile, err
	}
	return f.NewAPIClient(profile, credentials), profile, nil
}

// PublicAPIClient builds a client for operations that take no Access token,
// so it never reads stored credentials and works before `tmh login`.
func (f *Factory) PublicAPIClient() (*api.Client, error) {
	profile, err := f.requestProfile()
	if err != nil {
		return nil, err
	}
	return f.NewAPIClient(profile, config.Credentials{}), nil
}

// requestProfile is the selected profile, once the request flags are valid.
func (f *Factory) requestProfile() (config.Profile, error) {
	if f.maxRetries < 0 {
		return config.Profile{}, &UsageError{Message: "--max-retries must be 0 or more"}
	}
	return f.Profile()
}

func (f *Factory) NewAPIClient(profile config.Profile, credentials config.Credentials) *api.Client {
	var trace func(api.Exchange)
	if f.verbose || f.showHeaders {
		trace = func(exchange api.Exchange) { writeExchange(f.Stderr, exchange, f.showHeaders) }
	}
	var grant api.GrantStore
	if credentials.PartnerApp {
		grant = grantStore{store: f.Store(), profile: profile.Name}
	}
	return api.New(api.Config{
		Grant:          grant,
		BaseURL:        profile.BaseURL,
		ClientID:       credentials.ClientID,
		ClientSecret:   credentials.ClientSecret,
		OrganizationID: credentials.OrganizationID,
		Transport:      f.Transport,
		Clock:          f.Clock,
		UserAgent:      UserAgent(),
		Timeout:        80 * time.Second,
		MaxRetries:     f.maxRetries,
		Trace:          trace,
	})
}

// grantStore is a profile's Grant in the config store.
type grantStore struct {
	store   config.Store
	profile string
}

func (g grantStore) Lock(ctx context.Context) (func(), error) {
	return g.store.LockGrant(ctx, g.profile)
}
func (g grantStore) RefreshToken() (string, error) { return g.store.RefreshToken(g.profile) }
func (g grantStore) SaveRefreshToken(token string) error {
	return g.store.SaveRefreshToken(g.profile, token)
}

// UserAgent is the only thing tmh says about itself (ADR-0007).
func UserAgent() string {
	return fmt.Sprintf("teachmehipaa-cli/%s go/%s %s/%s",
		version.Version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

// PrintJSON writes an API response body to stdout; a 204's empty body
// prints nothing.
func (f *Factory) PrintJSON(body []byte) error {
	if len(body) == 0 {
		return nil
	}
	return output.JSON(f.Stdout, body, f.style())
}

// Items prints list items to stdout as they arrive: one JSON array, or
// NDJSON when ndjson is set.
func (f *Factory) Items(ndjson bool) *output.Items {
	if ndjson {
		return output.NewLines(f.Stdout, f.style())
	}
	return output.NewArray(f.Stdout, f.style())
}

func (f *Factory) style() output.Style {
	_, noColor := f.LookupEnv("NO_COLOR")
	return output.Style{Pretty: f.StdoutIsTTY, Color: f.StdoutIsTTY && !noColor}
}

// ReadLine reads one line from stdin without its line ending.
func (f *Factory) ReadLine() (string, error) {
	line, err := f.stdin.ReadString('\n')
	if err != nil && !(errors.Is(err, io.EOF) && line != "") {
		return "", err
	}
	return trimLineEnding(line), nil
}

// ReadAll reads the rest of stdin.
func (f *Factory) ReadAll() (string, error) {
	all, err := io.ReadAll(f.stdin)
	return string(all), err
}

// ReadHiddenSecret reads a secret typed at the terminal.
func (f *Factory) ReadHiddenSecret() (string, error) {
	if f.ReadSecret != nil {
		return f.ReadSecret()
	}
	return f.ReadLine()
}

func trimLineEnding(line string) string {
	for len(line) > 0 && (line[len(line)-1] == '\n' || line[len(line)-1] == '\r') {
		line = line[:len(line)-1]
	}
	return line
}
