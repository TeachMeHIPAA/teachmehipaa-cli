package commands_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/keyring"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
)

func TestLoginThenRetrieveALearner(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin string
		tty   bool
	}{
		{
			name:  "prompted for the client id and secret",
			args:  []string{"login"},
			stdin: testkit.ClientID + "\n" + testkit.ClientSecret + "\n",
			tty:   true,
		},
		{
			name:  "non-interactive, secret piped on stdin",
			args:  []string{"login", "--client-id", testkit.ClientID},
			stdin: testkit.ClientSecret + "\n",
		},
		{
			name:  "client id flag, secret prompted",
			args:  []string{"login", "--client-id", testkit.ClientID},
			stdin: testkit.ClientSecret + "\n",
			tty:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := testkit.NewEnv(t)
			env.Stdin, env.StdinIsTTY = tt.stdin, tt.tty
			env.Transport.
				Script(testkit.TokenRoute, testkit.TokenReply("jwt-login", 900), testkit.TokenReply("jwt-2", 900)).
				Script(testkit.IntrospectionRoute, testkit.IntrospectionReply("test")).
				Script(testkit.LearnerRoute, testkit.JSON(testkit.LearnerBody))

			login := env.Run(tt.args...)

			if login.ExitCode != 0 {
				t.Fatalf("login exit code = %d, stderr = %q", login.ExitCode, login.Stderr)
			}
			if want := "Logged in to Sandbox as Organization org_acme.\n"; login.Stdout != want {
				t.Errorf("login stdout = %q, want %q", login.Stdout, want)
			}
			secret, err := env.Keyring.Get("tmh", "sandbox")
			if err != nil || secret != testkit.ClientSecret {
				t.Errorf("keyring secret = %q, %v", secret, err)
			}
			config := readConfig(t, env)
			if strings.Contains(config, testkit.ClientSecret) {
				t.Errorf("config.toml holds the secret although the keyring works:\n%s", config)
			}

			env.Stdin = ""
			retrieve := env.Run("learners", "retrieve", testkit.LearnerID)

			if retrieve.ExitCode != 0 {
				t.Fatalf("retrieve exit code = %d, stderr = %q", retrieve.ExitCode, retrieve.Stderr)
			}
			if retrieve.Stdout != testkit.LearnerBody+"\n" {
				t.Errorf("retrieve stdout = %q", retrieve.Stdout)
			}
			// The Organization was learned at login, so no second introspection.
			wantRoutes := []string{testkit.TokenRoute, testkit.IntrospectionRoute, testkit.TokenRoute, testkit.LearnerRoute}
			if got := env.Transport.Routes(); strings.Join(got, ",") != strings.Join(wantRoutes, ",") {
				t.Errorf("routes = %v, want %v", got, wantRoutes)
			}
		})
	}
}

func TestLoginPromptsOnStderr(t *testing.T) {
	env := testkit.NewEnv(t)
	env.Stdin, env.StdinIsTTY = testkit.ClientID+"\n"+testkit.ClientSecret+"\n", true
	env.Transport.
		Script(testkit.TokenRoute, testkit.TokenReply("jwt-login", 900)).
		Script(testkit.IntrospectionRoute, testkit.IntrospectionReply("test"))

	result := env.Run("login")

	if want := "Client ID: Client secret: "; result.Stderr != want {
		t.Errorf("stderr = %q, want %q", result.Stderr, want)
	}
}

func TestLoginWithoutATerminalNeedsTheClientIDFlag(t *testing.T) {
	env := testkit.NewEnv(t)
	env.Stdin = testkit.ClientSecret + "\n"

	result := env.Run("login")

	if result.ExitCode != 1 {
		t.Errorf("exit code = %d, want 1 (usage)", result.ExitCode)
	}
	if !strings.Contains(result.Stderr, "--client-id") {
		t.Errorf("stderr = %q, want a hint about --client-id", result.Stderr)
	}
	if len(env.Transport.Requests()) != 0 {
		t.Errorf("routes = %v, want none", env.Transport.Routes())
	}
}

func TestLoginRejectedCredentialsAreNotStored(t *testing.T) {
	env := testkit.NewEnv(t)
	env.Stdin = testkit.ClientSecret
	env.Transport.Script(testkit.TokenRoute, testkit.Reply{
		Status: 401,
		Header: map[string]string{"WWW-Authenticate": `Basic realm="TeachMeHIPAA API"`, "X-Request-Id": "req-token-1"},
		Body:   `{"error":"invalid_client","error_description":"Client authentication failed."}`,
	})

	result := env.Run("login", "--client-id", testkit.ClientID)

	if result.ExitCode != 2 {
		t.Errorf("exit code = %d, want 2 (auth)", result.ExitCode)
	}
	want := "Error: invalid_client: Client authentication failed.\nRequest ID: req-token-1\n"
	if result.Stderr != want {
		t.Errorf("stderr = %q, want %q", result.Stderr, want)
	}
	if _, err := env.Keyring.Get("tmh", "sandbox"); err != keyring.ErrNotFound {
		t.Errorf("keyring holds a secret after a failed login (err = %v)", err)
	}
	if _, err := os.Stat(filepath.Join(env.ConfigDir, "config.toml")); !os.IsNotExist(err) {
		t.Errorf("config.toml written after a failed login (err = %v)", err)
	}
}

func TestLoginFallsBackToAPrivateFileWithoutAKeyring(t *testing.T) {
	env := testkit.NewEnv(t)
	env.Keyring = keyring.NewUnavailable()
	env.Stdin = testkit.ClientSecret
	env.Transport.
		Script(testkit.TokenRoute, testkit.TokenReply("jwt-login", 900), testkit.TokenReply("jwt-2", 900)).
		Script(testkit.IntrospectionRoute, testkit.IntrospectionReply("test")).
		Script(testkit.LearnerRoute, testkit.JSON(testkit.LearnerBody))

	login := env.Run("login", "--client-id", testkit.ClientID)

	if login.ExitCode != 0 {
		t.Fatalf("login exit code = %d, stderr = %q", login.ExitCode, login.Stderr)
	}
	path := filepath.Join(env.ConfigDir, "config.toml")
	if !strings.Contains(login.Stderr, "OS keyring unavailable") || !strings.Contains(login.Stderr, path) {
		t.Errorf("login stderr = %q, want a note naming %s", login.Stderr, path)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("config.toml mode = %o, want 600", mode)
	}
	if config := readConfig(t, env); !strings.Contains(config, testkit.ClientSecret) {
		t.Errorf("config.toml lacks the secret:\n%s", config)
	}

	retrieve := env.Run("learners", "retrieve", testkit.LearnerID)

	if retrieve.ExitCode != 0 {
		t.Fatalf("retrieve exit code = %d, stderr = %q", retrieve.ExitCode, retrieve.Stderr)
	}
	if retrieve.Stdout != testkit.LearnerBody+"\n" {
		t.Errorf("retrieve stdout = %q", retrieve.Stdout)
	}
}

func readConfig(t *testing.T, env *testkit.Env) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(env.ConfigDir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestLoginTakesOnlyAnAPIClient(t *testing.T) {
	for _, flag := range []string{"--partner-app", "--scopes=learners:read", "--redirect-port=8765", "--authorize-url=https://example.com"} {
		t.Run(flag, func(t *testing.T) {
			env := testkit.NewEnv(t)
			env.Stdin = testkit.ClientSecret

			result := env.Run("login", "--client-id", testkit.ClientID, flag)

			if result.ExitCode != 1 {
				t.Errorf("exit code = %d, want 1 (usage)", result.ExitCode)
			}
			name, _, _ := strings.Cut(flag, "=")
			if !strings.Contains(result.Stderr, "unknown flag: "+name) {
				t.Errorf("stderr = %q, want an unknown flag error for %s", result.Stderr, name)
			}
			if routes := env.Transport.Routes(); len(routes) != 0 {
				t.Errorf("routes = %v, want none", routes)
			}
		})
	}
}

// A config.toml written by an older tmh may hold keys this one no longer
// knows, such as a Partner app profile's, and must keep working.
func TestAConfigFileWithUnknownKeysStillWorks(t *testing.T) {
	env := testkit.NewEnv(t)
	config := `future_setting = "on"

[profiles.sandbox]
client_id = "` + testkit.ClientID + `"
organization_id = "` + testkit.OrgID + `"
partner_app = true
client_secret = "` + testkit.ClientSecret + `"
refresh_token = "tmh_rt_grt_01JD8A2B3C4D5E6F7G8H9J0K1M.old-entropy"
another_future_key = 3

[future_table]
key = "value"
`
	if err := os.WriteFile(filepath.Join(env.ConfigDir, "config.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	env.Transport.
		Script(testkit.TokenRoute, testkit.TokenReply("jwt-1", 900), testkit.TokenReply("jwt-2", 900)).
		Script(testkit.IntrospectionRoute, testkit.IntrospectionReply("test")).
		Script(testkit.LearnerRoute, testkit.JSON(testkit.LearnerBody))

	retrieve := env.Run("learners", "retrieve", testkit.LearnerID)

	if retrieve.ExitCode != 0 {
		t.Fatalf("retrieve exit code = %d, stderr = %q", retrieve.ExitCode, retrieve.Stderr)
	}
	if token := env.Transport.Requests()[0]; token.Body != "grant_type=client_credentials" {
		t.Errorf("token request body = %q, want the client-credentials grant", token.Body)
	}

	env.Stdin = testkit.ClientSecret
	login := env.Run("login", "--client-id", testkit.ClientID)

	if login.ExitCode != 0 {
		t.Fatalf("login exit code = %d, stderr = %q", login.ExitCode, login.Stderr)
	}
	if rewritten := readConfig(t, env); strings.Contains(rewritten, "refresh_token") || strings.Contains(rewritten, "partner_app") {
		t.Errorf("login kept the old Partner app keys:\n%s", rewritten)
	}
}
