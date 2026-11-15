package commands_test

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/keyring"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
)

// loginWithGrant leaves the sandbox profile holding a Grant whose Refresh
// token is RefreshToken1.
func loginWithGrant(t *testing.T, env *testkit.Env) {
	t.Helper()
	env.Stdin = testkit.PartnerAppSecret
	consentingBrowser(t, env, nil)
	env.Transport.
		Script(testkit.TokenRoute, testkit.GrantTokenReply("jwt-grant-login", testkit.RefreshToken1)).
		Script(testkit.IntrospectionRoute, testkit.GrantIntrospectionReply())
	result := env.Run("login", "--partner-app", "--client-id", testkit.PartnerAppID, "--authorize-url", sandboxConsentURL)
	if result.ExitCode != 0 {
		t.Fatalf("login exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	env.Stdin, env.Browser = "", nil
}

// commandRequests drops the login's requests.
func commandRequests(env *testkit.Env) []testkit.Request {
	return env.Transport.Requests()[2:]
}

func refreshTokenPresented(t *testing.T, r testkit.Request) string {
	t.Helper()
	form, err := url.ParseQuery(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	if got := form.Get("grant_type"); got != "refresh_token" {
		t.Errorf("token request grant_type = %q, want refresh_token", got)
	}
	return form.Get("refresh_token")
}

func storedRefreshToken(env *testkit.Env) string {
	token, _ := env.Keyring.Get("tmh", "sandbox:refresh-token")
	return token
}

func TestAGrantStoresTheRotatedRefreshTokenBeforeTheRequestUsesIt(t *testing.T) {
	env := testkit.NewEnv(t)
	loginWithGrant(t, env)
	var storedWhenRequestSent string
	learner := testkit.JSON(testkit.LearnerBody)
	learner.Before = func() { storedWhenRequestSent = storedRefreshToken(env) }
	env.Transport.
		Script(testkit.TokenRoute, testkit.GrantTokenReply("jwt-grant-2", testkit.RefreshToken2)).
		Script(testkit.LearnerRoute, learner)

	result := env.Run("learners", "retrieve", testkit.LearnerID)

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if result.Stdout != testkit.LearnerBody+"\n" {
		t.Errorf("stdout = %q", result.Stdout)
	}
	requests := commandRequests(env)
	if got := routesOf(requests); got != testkit.TokenRoute+","+testkit.LearnerRoute {
		t.Fatalf("routes = %s, want one refresh then the request", got)
	}
	refresh := requests[0]
	if id, secret, _ := (&http.Request{Header: refresh.Header}).BasicAuth(); id != testkit.PartnerAppID || secret != testkit.PartnerAppSecret {
		t.Errorf("refresh Basic credentials = %q:%q, want the Partner app's", id, secret)
	}
	if got := refreshTokenPresented(t, refresh); got != testkit.RefreshToken1 {
		t.Errorf("refresh presented %q, want the stored %q", got, testkit.RefreshToken1)
	}
	if storedWhenRequestSent != testkit.RefreshToken2 {
		t.Errorf("stored Refresh token when the request left = %q, want the rotated %q", storedWhenRequestSent, testkit.RefreshToken2)
	}
	assertHeader(t, requests[1], "Authorization", "Bearer jwt-grant-2")

	env.Transport.
		Script(testkit.TokenRoute, testkit.GrantTokenReply("jwt-grant-3", testkit.RefreshToken3)).
		Script(testkit.LearnerRoute, testkit.JSON(testkit.LearnerBody))
	if next := env.Run("learners", "retrieve", testkit.LearnerID); next.ExitCode != 0 {
		t.Fatalf("next command exit code = %d, stderr = %q", next.ExitCode, next.Stderr)
	}
	if got := refreshTokenPresented(t, commandRequests(env)[2]); got != testkit.RefreshToken2 {
		t.Errorf("next command presented %q, want the rotated %q", got, testkit.RefreshToken2)
	}
}

func TestAGrantWhoseRotatedRefreshTokenCannotBeStoredSendsNoRequest(t *testing.T) {
	env := testkit.NewEnv(t)
	env.Keyring = keyring.NewUnavailable()
	loginWithGrant(t, env)
	env.Transport.
		Script(testkit.TokenRoute, testkit.GrantTokenReply("jwt-grant-2", testkit.RefreshToken2)).
		Script(testkit.LearnerRoute, testkit.JSON(testkit.LearnerBody))
	if first := env.Run("learners", "retrieve", testkit.LearnerID); first.ExitCode != 0 {
		t.Fatalf("first command exit code = %d, stderr = %q", first.ExitCode, first.Stderr)
	}
	// A read-only config dir makes the file fallback fail too.
	if err := os.Chmod(env.ConfigDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(env.ConfigDir, 0o700) })
	env.Transport.Script(testkit.TokenRoute, testkit.GrantTokenReply("jwt-grant-3", testkit.RefreshToken3))

	result := env.Run("learners", "retrieve", testkit.LearnerID)

	if result.ExitCode == 0 {
		t.Fatalf("exit code = 0, stdout = %q", result.Stdout)
	}
	if !strings.Contains(result.Stderr, "could not store the rotated Refresh token, so its Access token was not used") ||
		!strings.Contains(result.Stderr, "run `tmh login --partner-app` again") {
		t.Errorf("stderr = %q", result.Stderr)
	}
	if got := routesOf(commandRequests(env)[2:]); got != testkit.TokenRoute {
		t.Errorf("routes = %s, want the refresh and no API request", got)
	}
}

func TestAFailedRefreshIsNeverRetried(t *testing.T) {
	tests := []struct {
		name     string
		reply    testkit.Reply
		wantExit int
		want     string
	}{
		{
			name:     "network error",
			reply:    testkit.Reply{Err: errors.New("connection reset by peer")},
			wantExit: 7,
			want: "The Refresh token exchange was not retried, because replaying a Refresh token the API may already have rotated revokes the Grant. " +
				"If the next command fails with invalid_grant, run `tmh login --partner-app` again\n",
		},
		{
			name: "temporarily unavailable",
			reply: testkit.Reply{
				Status: 503,
				Body:   `{"error":"temporarily_unavailable","error_description":"Try again shortly."}`,
			},
			wantExit: 7,
			want:     "Error: temporarily_unavailable: Try again shortly.\n",
		},
		{
			name: "revoked or reused Grant",
			reply: testkit.Reply{
				Status: 400,
				Header: map[string]string{"X-Request-Id": "req-refresh-1"},
				Body:   `{"error":"invalid_grant","error_description":"The refresh token is invalid."}`,
			},
			wantExit: 2,
			want: "Error: invalid_grant: The refresh token is invalid. The Grant was revoked, or its Refresh token was reused " +
				"(presenting a superseded Refresh token revokes the Grant). Run `tmh login --partner-app` again\nRequest ID: req-refresh-1\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := testkit.NewEnv(t)
			loginWithGrant(t, env)
			env.Transport.Script(testkit.TokenRoute, tt.reply, testkit.GrantTokenReply("jwt-never", testkit.RefreshToken2))

			result := env.Run("learners", "retrieve", testkit.LearnerID)

			if result.ExitCode != tt.wantExit {
				t.Errorf("exit code = %d, want %d", result.ExitCode, tt.wantExit)
			}
			if !strings.HasSuffix(result.Stderr, tt.want) {
				t.Errorf("stderr = %q, want it to end %q", result.Stderr, tt.want)
			}
			if got := routesOf(commandRequests(env)); got != testkit.TokenRoute {
				t.Errorf("routes = %s, want exactly one exchange", got)
			}
			if got := storedRefreshToken(env); got != testkit.RefreshToken1 {
				t.Errorf("stored Refresh token = %q, want it unchanged", got)
			}
		})
	}
}

func routesOf(requests []testkit.Request) string {
	routes := make([]string, 0, len(requests))
	for _, r := range requests {
		routes = append(routes, r.Method+" "+r.Path)
	}
	return strings.Join(routes, ",")
}

// Two tmh processes on one machine must never present the same Refresh
// token: the second waits for the first's exchange, then reads the token it
// stored.
func TestConcurrentRefreshesOfOneGrantAreSerialized(t *testing.T) {
	first := testkit.NewEnv(t)
	loginWithGrant(t, first)
	second := testkit.NewEnv(t)
	second.ConfigDir, second.Keyring = first.ConfigDir, first.Keyring

	exchanging, release := make(chan struct{}), make(chan struct{})
	held := testkit.GrantTokenReply("jwt-first", testkit.RefreshToken2)
	held.Before = func() {
		close(exchanging)
		<-release
	}
	first.Transport.Script(testkit.TokenRoute, held).Script(testkit.LearnerRoute, testkit.JSON(testkit.LearnerBody))
	second.Transport.
		Script(testkit.TokenRoute, testkit.GrantTokenReply("jwt-second", testkit.RefreshToken3)).
		Script(testkit.LearnerRoute, testkit.JSON(testkit.LearnerBody))

	results := make(chan testkit.Result, 2)
	go func() { results <- first.Run("learners", "retrieve", testkit.LearnerID) }()
	<-exchanging
	go func() { results <- second.Run("learners", "retrieve", testkit.LearnerID) }()
	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) && len(second.Transport.Requests()) == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if got := second.Transport.Routes(); len(got) != 0 {
		t.Errorf("the second process sent %v while the first was exchanging", got)
	}
	close(release)
	for range 2 {
		if result := <-results; result.ExitCode != 0 {
			t.Errorf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
		}
	}

	if got := refreshTokenPresented(t, commandRequests(first)[0]); got != testkit.RefreshToken1 {
		t.Errorf("first process presented %q, want %q", got, testkit.RefreshToken1)
	}
	if got := refreshTokenPresented(t, second.Transport.Requests()[0]); got != testkit.RefreshToken2 {
		t.Errorf("second process presented %q, want the first's rotated %q", got, testkit.RefreshToken2)
	}
	if got := storedRefreshToken(first); got != testkit.RefreshToken3 {
		t.Errorf("stored Refresh token = %q, want the latest %q", got, testkit.RefreshToken3)
	}
}

func TestWhoamiShowsAGrantsPartnerApp(t *testing.T) {
	env := testkit.NewEnv(t)
	loginWithGrant(t, env)
	env.Transport.
		Script(testkit.TokenRoute, testkit.GrantTokenReply("jwt-grant-2", testkit.RefreshToken2)).
		Script(testkit.IntrospectionRoute, testkit.GrantIntrospectionReply())

	result := env.Run("whoami")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	want := "Organization  org_acme\nScopes        learners:read, learners:write\nEnvironment   Sandbox\n" +
		"Partner app   " + testkit.PartnerAppID + " (Grant)\n"
	if result.Stdout != want {
		t.Errorf("stdout = %q, want %q", result.Stdout, want)
	}
}

func TestLogoutClearsTheGrant(t *testing.T) {
	tests := []struct {
		name    string
		keyring keyring.Keyring
	}{
		{name: "stored in the OS keyring", keyring: keyring.NewMemory()},
		{name: "stored in the config file", keyring: keyring.NewUnavailable()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := testkit.NewEnv(t)
			env.Keyring = tt.keyring
			loginWithGrant(t, env)

			result := env.Run("logout")

			if result.ExitCode != 0 || result.Stdout != "Logged out of the sandbox profile.\n" {
				t.Fatalf("logout = %d, %q, %q", result.ExitCode, result.Stdout, result.Stderr)
			}
			if got := storedRefreshToken(env); got != "" {
				t.Errorf("the keyring still holds Refresh token %q", got)
			}
			if config := readConfig(t, env); strings.Contains(config, testkit.RefreshToken1) || strings.Contains(config, testkit.PartnerAppSecret) {
				t.Errorf("config.toml still holds the Grant:\n%s", config)
			}
			retrieve := env.Run("learners", "retrieve", testkit.LearnerID)
			if retrieve.ExitCode != 2 || len(commandRequests(env)) != 0 {
				t.Errorf("after logout: exit %d, routes %v; want 2 and no request", retrieve.ExitCode, env.Transport.Routes())
			}
		})
	}
}
