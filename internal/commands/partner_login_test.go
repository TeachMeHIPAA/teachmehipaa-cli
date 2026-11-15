package commands_test

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
)

const sandboxConsentURL = "https://console-staging.example.test/oauth/authorize"

// browserVisit is what the fake browser saw when it followed the consent
// redirect back to tmh.
type browserVisit struct {
	authorizeURL *url.URL
	status       int
	page         string
}

// consentingBrowser plays an admin who grants access: it follows the
// consent URL's redirect_uri back to tmh's loopback server with a code and
// the state, after redirect changes the callback query if set.
func consentingBrowser(t *testing.T, env *testkit.Env, redirect func(query url.Values)) *browserVisit {
	t.Helper()
	visit := &browserVisit{}
	env.Browser = func(rawURL string) error {
		authorize, err := url.Parse(rawURL)
		if err != nil {
			t.Errorf("consent URL %q: %v", rawURL, err)
			return nil
		}
		visit.authorizeURL = authorize
		query := url.Values{"code": {testkit.AuthorizationCode}, "state": {authorize.Query().Get("state")}}
		if redirect != nil {
			redirect(query)
		}
		client := &http.Client{Timeout: 5 * time.Second}
		response, err := client.Get(authorize.Query().Get("redirect_uri") + "?" + query.Encode())
		if err != nil {
			t.Errorf("following the redirect to tmh: %v", err)
			return nil
		}
		defer response.Body.Close()
		page, _ := io.ReadAll(response.Body)
		visit.status, visit.page = response.StatusCode, string(page)
		return nil
	}
	return visit
}

func TestPartnerAppLoginRunsTheConsentFlowAndStoresTheGrant(t *testing.T) {
	env := testkit.NewEnv(t)
	env.Stdin = testkit.PartnerAppSecret
	visit := consentingBrowser(t, env, nil)
	env.Transport.
		Script(testkit.TokenRoute, testkit.GrantTokenReply("jwt-grant-1", testkit.RefreshToken1)).
		Script(testkit.IntrospectionRoute, testkit.GrantIntrospectionReply())

	result := env.Run("login", "--partner-app", "--live", "--client-id", testkit.PartnerAppID,
		"--scopes", "learners:read,learners:write")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if want := "Logged in to Live as Organization org_acme through Partner app " + testkit.PartnerAppID + ".\n"; result.Stdout != want {
		t.Errorf("stdout = %q, want %q", result.Stdout, want)
	}

	consent := visit.authorizeURL
	if consent == nil {
		t.Fatal("no browser was opened")
	}
	if got := consent.Scheme + "://" + consent.Host + consent.Path; got != "https://console.teachmehipaa.com/oauth/authorize" {
		t.Errorf("consent URL = %s, want the Live console's", got)
	}
	query := consent.Query()
	for name, want := range map[string]string{
		"response_type":         "code",
		"client_id":             testkit.PartnerAppID,
		"scope":                 "learners:read learners:write",
		"code_challenge_method": "S256",
	} {
		if got := query.Get(name); got != want {
			t.Errorf("consent URL %s = %q, want %q", name, got, want)
		}
	}
	if strings.Contains(consent.RawQuery, "+") {
		t.Errorf("consent URL query %q encodes spaces as +, which only form parsers read back", consent.RawQuery)
	}
	redirectURI := query.Get("redirect_uri")
	if !regexp.MustCompile(`^http://127\.0\.0\.1:[1-9][0-9]*/callback$`).MatchString(redirectURI) {
		t.Errorf("redirect_uri = %q, want http://127.0.0.1:<port>/callback", redirectURI)
	}
	if len(query.Get("state")) < 32 {
		t.Errorf("state = %q, want at least 32 random characters", query.Get("state"))
	}
	if visit.status != http.StatusOK {
		t.Errorf("callback page status = %d, want 200", visit.status)
	}

	requests := env.Transport.Requests()
	exchange := requests[0]
	if got := exchange.Method + " " + exchange.Path; got != testkit.TokenRoute {
		t.Fatalf("first request = %s, want the code exchange", got)
	}
	if !strings.HasPrefix(exchange.URL, "https://api.tme.so/v1/") {
		t.Errorf("code exchanged at %s, want the Live API", exchange.URL)
	}
	if id, secret, _ := (&http.Request{Header: exchange.Header}).BasicAuth(); id != testkit.PartnerAppID || secret != testkit.PartnerAppSecret {
		t.Errorf("code exchange Basic credentials = %q:%q, want the Partner app's", id, secret)
	}
	form, err := url.ParseQuery(exchange.Body)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"grant_type":   "authorization_code",
		"code":         testkit.AuthorizationCode,
		"redirect_uri": redirectURI,
	} {
		if got := form.Get(name); got != want {
			t.Errorf("code exchange %s = %q, want %q", name, got, want)
		}
	}
	// RFC 7636 §4.1–4.2: the verifier is 43–128 unreserved characters and
	// the challenge is BASE64URL(SHA256(verifier)) without padding.
	verifier := form.Get("code_verifier")
	if !regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`).MatchString(verifier) {
		t.Errorf("code_verifier = %q, want 43–128 unreserved characters", verifier)
	}
	digest := sha256.Sum256([]byte(verifier))
	if want := base64.RawURLEncoding.EncodeToString(digest[:]); query.Get("code_challenge") != want {
		t.Errorf("code_challenge = %q, want S256 of the verifier, %q", query.Get("code_challenge"), want)
	}
	assertHeader(t, requests[1], "Authorization", "Bearer jwt-grant-1")

	if got, err := env.Keyring.Get("tmh", "live:refresh-token"); err != nil || got != testkit.RefreshToken1 {
		t.Errorf("keyring Refresh token = %q, %v; want the Grant's", got, err)
	}
	if got, err := env.Keyring.Get("tmh", "live"); err != nil || got != testkit.PartnerAppSecret {
		t.Errorf("keyring app secret = %q, %v", got, err)
	}
	config := readConfig(t, env)
	if strings.Contains(config, testkit.RefreshToken1) || strings.Contains(config, testkit.PartnerAppSecret) {
		t.Errorf("config.toml holds a secret although the keyring works:\n%s", config)
	}
}

func TestPartnerAppLoginToSandboxNeedsAnAuthorizeURL(t *testing.T) {
	env := testkit.NewEnv(t)
	env.Stdin = testkit.PartnerAppSecret

	result := env.Run("login", "--partner-app", "--client-id", testkit.PartnerAppID)

	if result.ExitCode != 1 {
		t.Errorf("exit code = %d, want 1 (usage)", result.ExitCode)
	}
	if want := "Error: the sandbox profile has no published consent page: pass --authorize-url\n"; result.Stderr != want {
		t.Errorf("stderr = %q, want %q", result.Stderr, want)
	}
	if len(env.Opened) != 0 || len(env.Transport.Requests()) != 0 {
		t.Errorf("opened %v and sent %v, want nothing", env.Opened, env.Transport.Routes())
	}
}

func TestPartnerAppLoginToSandboxOpensTheGivenAuthorizeURL(t *testing.T) {
	env := testkit.NewEnv(t)
	env.Stdin = testkit.PartnerAppSecret
	visit := consentingBrowser(t, env, nil)
	env.Transport.
		Script(testkit.TokenRoute, testkit.GrantTokenReply("jwt-grant-1", testkit.RefreshToken1)).
		Script(testkit.IntrospectionRoute, testkit.GrantIntrospectionReply())

	result := env.Run("login", "--partner-app", "--client-id", testkit.PartnerAppID, "--authorize-url", sandboxConsentURL)

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if got := visit.authorizeURL.Scheme + "://" + visit.authorizeURL.Host + visit.authorizeURL.Path; got != sandboxConsentURL {
		t.Errorf("consent URL = %s, want %s", got, sandboxConsentURL)
	}
	if exchange := env.Transport.Requests()[0]; !strings.HasPrefix(exchange.URL, "https://api-teachmehipaa-dev.web.app/v1/") {
		t.Errorf("code exchanged at %s, want the Sandbox API", exchange.URL)
	}
	if want := "Logged in to Sandbox as Organization org_acme through Partner app " + testkit.PartnerAppID + ".\n"; result.Stdout != want {
		t.Errorf("stdout = %q, want %q", result.Stdout, want)
	}
}

func TestPartnerAppLoginListensOnTheGivenRedirectPort(t *testing.T) {
	port := freePort(t)
	env := testkit.NewEnv(t)
	env.Stdin = testkit.PartnerAppSecret
	visit := consentingBrowser(t, env, nil)
	env.Transport.
		Script(testkit.TokenRoute, testkit.GrantTokenReply("jwt-grant-1", testkit.RefreshToken1)).
		Script(testkit.IntrospectionRoute, testkit.GrantIntrospectionReply())

	result := env.Run("login", "--partner-app", "--client-id", testkit.PartnerAppID,
		"--authorize-url", sandboxConsentURL, "--redirect-port", strconv.Itoa(port))

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	want := "http://127.0.0.1:" + strconv.Itoa(port) + "/callback"
	if got := visit.authorizeURL.Query().Get("redirect_uri"); got != want {
		t.Errorf("redirect_uri = %q, want %q", got, want)
	}
}

func TestPartnerAppLoginRejectsARedirectWithTheWrongState(t *testing.T) {
	env := testkit.NewEnv(t)
	env.Stdin = testkit.PartnerAppSecret
	visit := consentingBrowser(t, env, func(query url.Values) { query.Set("state", "forged-state") })

	result := env.Run("login", "--partner-app", "--client-id", testkit.PartnerAppID, "--authorize-url", sandboxConsentURL)

	if result.ExitCode != 2 {
		t.Errorf("exit code = %d, want 2 (auth)", result.ExitCode)
	}
	if !strings.Contains(result.Stderr, "state does not match") {
		t.Errorf("stderr = %q, want a state mismatch", result.Stderr)
	}
	if visit.status != http.StatusBadRequest {
		t.Errorf("callback page status = %d, want 400", visit.status)
	}
	if len(env.Transport.Requests()) != 0 {
		t.Errorf("routes = %v, want no code exchange", env.Transport.Routes())
	}
	assertNoGrantStored(t, env)
}

func TestPartnerAppLoginReportsADeniedConsent(t *testing.T) {
	env := testkit.NewEnv(t)
	env.Stdin = testkit.PartnerAppSecret
	consentingBrowser(t, env, func(query url.Values) {
		query.Del("code")
		query.Set("error", "access_denied")
		query.Set("error_description", "The admin declined.")
	})

	result := env.Run("login", "--partner-app", "--client-id", testkit.PartnerAppID, "--authorize-url", sandboxConsentURL)

	if result.ExitCode != 2 {
		t.Errorf("exit code = %d, want 2 (auth)", result.ExitCode)
	}
	if !strings.HasSuffix(result.Stderr, "Error: consent was not granted: access_denied: The admin declined.\n") {
		t.Errorf("stderr = %q", result.Stderr)
	}
	if len(env.Transport.Requests()) != 0 {
		t.Errorf("routes = %v, want no code exchange", env.Transport.Routes())
	}
}

func TestPartnerAppLoginCodeExchangeFailures(t *testing.T) {
	tests := []struct {
		name     string
		reply    testkit.Reply
		wantExit int
		want     string
	}{
		{
			name: "OAuth error",
			reply: testkit.Reply{
				Status: 400,
				Header: map[string]string{"X-Request-Id": "req-code-1"},
				Body:   `{"error":"invalid_grant","error_description":"The authorization code is invalid or expired."}`,
			},
			wantExit: 2,
			want:     "Error: invalid_grant: The authorization code is invalid or expired.",
		},
		{
			name:     "network error",
			reply:    testkit.Reply{Err: errors.New("connection reset by peer")},
			wantExit: 7,
			want:     "the code may have been consumed, and a code works only once. Run `tmh login --partner-app` again",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := testkit.NewEnv(t)
			env.Stdin = testkit.PartnerAppSecret
			consentingBrowser(t, env, nil)
			env.Transport.Script(testkit.TokenRoute, tt.reply, testkit.GrantTokenReply("jwt-never", testkit.RefreshToken2))

			result := env.Run("login", "--partner-app", "--client-id", testkit.PartnerAppID, "--authorize-url", sandboxConsentURL)

			if result.ExitCode != tt.wantExit {
				t.Errorf("exit code = %d, want %d; stderr = %q", result.ExitCode, tt.wantExit, result.Stderr)
			}
			if !strings.Contains(result.Stderr, tt.want) {
				t.Errorf("stderr = %q, want it to contain %q", result.Stderr, tt.want)
			}
			if got := env.Transport.Routes(); len(got) != 1 {
				t.Errorf("routes = %v, want exactly one code exchange", got)
			}
			assertNoGrantStored(t, env)
		})
	}
}

func assertNoGrantStored(t *testing.T, env *testkit.Env) {
	t.Helper()
	if _, err := env.Keyring.Get("tmh", "sandbox:refresh-token"); err == nil {
		t.Error("the keyring holds a Refresh token")
	}
	if _, err := os.Stat(filepath.Join(env.ConfigDir, "config.toml")); !os.IsNotExist(err) {
		t.Errorf("config.toml written (err = %v)", err)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func TestPartnerAppFlagsNeedPartnerApp(t *testing.T) {
	env := testkit.NewEnv(t)
	env.Stdin = testkit.ClientSecret

	result := env.Run("login", "--client-id", testkit.ClientID, "--scopes", "learners:read")

	if result.ExitCode != 1 || result.Stderr != "Error: --scopes needs --partner-app\n" {
		t.Errorf("exit code = %d, stderr = %q; want 1 and a hint", result.ExitCode, result.Stderr)
	}
	if len(env.Transport.Requests()) != 0 {
		t.Errorf("routes = %v, want none", env.Transport.Routes())
	}
}
