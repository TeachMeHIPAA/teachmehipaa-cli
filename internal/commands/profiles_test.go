package commands_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
)

const liveSecret = "tmh_cs_bGl2ZS1saXZlLWxpdmUtbGl2ZS1saXZlLWxpdmUtbGl2ZS0"

func TestProfilesSelectTheEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		flags    []string
		wantHost string
		wantAuth string
	}{
		{name: "sandbox by default", flags: nil, wantHost: "https://api-teachmehipaa-dev.web.app/v1/", wantAuth: testkit.ClientSecret},
		{name: "--profile sandbox", flags: []string{"--profile", "sandbox"}, wantHost: "https://api-teachmehipaa-dev.web.app/v1/", wantAuth: testkit.ClientSecret},
		{name: "--live", flags: []string{"--live"}, wantHost: "https://api.tme.so/v1/", wantAuth: liveSecret},
		{name: "--profile live", flags: []string{"--profile", "live"}, wantHost: "https://api.tme.so/v1/", wantAuth: liveSecret},
		{name: "--live --profile live", flags: []string{"--live", "--profile", "live"}, wantHost: "https://api.tme.so/v1/", wantAuth: liveSecret},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := testkit.NewEnv(t)
			loginTo(t, env, nil, testkit.ClientSecret)
			loginTo(t, env, []string{"--live"}, liveSecret)
			env.Transport.
				Script(testkit.TokenRoute, testkit.TokenReply("jwt-cmd", 900)).
				Script(testkit.LearnerRoute, testkit.JSON(testkit.LearnerBody))

			result := env.Run(append([]string{"learners", "retrieve", testkit.LearnerID}, tt.flags...)...)

			if result.ExitCode != 0 {
				t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
			}
			requests := env.Transport.Requests()
			token, learner := requests[len(requests)-2], requests[len(requests)-1]
			if !strings.HasPrefix(token.URL, tt.wantHost) || !strings.HasPrefix(learner.URL, tt.wantHost) {
				t.Errorf("URLs = %s, %s, want prefix %s", token.URL, learner.URL, tt.wantHost)
			}
			if got := basicSecret(t, token); got != tt.wantAuth {
				t.Errorf("token minted with secret %q, want %q", got, tt.wantAuth)
			}
		})
	}
}

func TestProfileFlagMistakesAreUsageErrors(t *testing.T) {
	tests := []struct {
		name  string
		flags []string
		want  string
	}{
		{name: "unknown profile", flags: []string{"--profile", "staging"}, want: `Error: unknown profile "staging": use "sandbox" or "live"` + "\n"},
		{name: "--live with --profile sandbox", flags: []string{"--live", "--profile", "sandbox"}, want: "Error: --live conflicts with --profile sandbox\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := testkit.NewEnv(t)

			result := env.Run(append([]string{"learners", "retrieve", testkit.LearnerID}, tt.flags...)...)

			if result.ExitCode != 1 {
				t.Errorf("exit code = %d, want 1", result.ExitCode)
			}
			if result.Stderr != tt.want {
				t.Errorf("stderr = %q, want %q", result.Stderr, tt.want)
			}
			if len(env.Transport.Requests()) != 0 {
				t.Errorf("routes = %v, want none", env.Transport.Routes())
			}
		})
	}
}

func TestEnvironmentCredentialsOverrideTheStoredOnes(t *testing.T) {
	env := testkit.NewEnv(t)
	loginTo(t, env, nil, testkit.ClientSecret)
	const ciID, ciSecret = "cli_01JD9CICICICICICICICICICIC", "tmh_cs_Y2ktY2ktY2ktY2ktY2ktY2ktY2ktY2ktY2ktY2ktY2k"
	env.Vars["TMH_CLIENT_ID"], env.Vars["TMH_CLIENT_SECRET"] = ciID, ciSecret
	env.Transport.
		Script(testkit.TokenRoute, testkit.TokenReply("jwt-ci", 900)).
		Script(testkit.IntrospectionRoute, testkit.IntrospectionReply("test")).
		Script(testkit.LearnerRoute, testkit.JSON(testkit.LearnerBody))

	result := env.Run("learners", "retrieve", testkit.LearnerID)

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	routes := env.Transport.Routes()[2:]
	want := []string{testkit.TokenRoute, testkit.IntrospectionRoute, testkit.LearnerRoute}
	if strings.Join(routes, ",") != strings.Join(want, ",") {
		t.Errorf("routes = %v, want %v (the stored Organization belongs to other credentials)", routes, want)
	}
	if got := basicSecret(t, env.Transport.Requests()[2]); got != ciSecret {
		t.Errorf("token minted with secret %q, want the TMH_CLIENT_SECRET one", got)
	}
}

func TestOnlyOneEnvironmentCredentialIsAUsageError(t *testing.T) {
	env := testkit.NewEnv(t)
	env.Vars["TMH_CLIENT_ID"] = testkit.ClientID

	result := env.Run("learners", "retrieve", testkit.LearnerID)

	if result.ExitCode != 1 {
		t.Errorf("exit code = %d, want 1", result.ExitCode)
	}
	if want := "Error: set both TMH_CLIENT_ID and TMH_CLIENT_SECRET, or neither\n"; result.Stderr != want {
		t.Errorf("stderr = %q, want %q", result.Stderr, want)
	}
}

// loginTo runs a non-interactive login, scripting its token and
// introspection calls.
func loginTo(t *testing.T, env *testkit.Env, flags []string, secret string) {
	t.Helper()
	env.Stdin = secret
	env.Transport.
		Script(testkit.TokenRoute, testkit.TokenReply("jwt-login", 900)).
		Script(testkit.IntrospectionRoute, testkit.IntrospectionReply("test"))
	result := env.Run(append([]string{"login", "--client-id", testkit.ClientID}, flags...)...)
	if result.ExitCode != 0 {
		t.Fatalf("login exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	env.Stdin = ""
}

func basicSecret(t *testing.T, r testkit.Request) string {
	t.Helper()
	_, secret, ok := (&http.Request{Header: r.Header}).BasicAuth()
	if !ok {
		t.Fatalf("%s %s has no Basic credentials", r.Method, r.Path)
	}
	return secret
}
