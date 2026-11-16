package commands_test

import (
	"strings"
	"testing"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/keyring"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
)

func TestLogoutRemovesTheProfilesCredentials(t *testing.T) {
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
			loginTo(t, env, nil, testkit.ClientSecret)
			loginTo(t, env, []string{"--live"}, liveSecret)

			logout := env.Run("logout")

			if logout.ExitCode != 0 {
				t.Fatalf("logout exit code = %d, stderr = %q", logout.ExitCode, logout.Stderr)
			}
			if want := "Logged out of the sandbox profile.\n"; logout.Stdout != want {
				t.Errorf("logout stdout = %q, want %q", logout.Stdout, want)
			}
			if _, err := env.Keyring.Get("tmh", "sandbox"); err == nil {
				t.Error("the keyring still holds the sandbox secret")
			}
			if config := readConfig(t, env); strings.Contains(config, "sandbox") || strings.Contains(config, testkit.ClientSecret) {
				t.Errorf("config.toml still has the sandbox profile:\n%s", config)
			}

			retrieve := env.Run("learners", "retrieve", testkit.LearnerID)

			if retrieve.ExitCode != 2 {
				t.Errorf("retrieve exit code = %d, want 2 (auth)", retrieve.ExitCode)
			}
			want := "Error: not logged in to the sandbox profile; run `tmh login` or set TMH_CLIENT_ID and TMH_CLIENT_SECRET\n"
			if retrieve.Stderr != want {
				t.Errorf("retrieve stderr = %q, want %q", retrieve.Stderr, want)
			}

			env.Transport.
				Script(testkit.TokenRoute, testkit.TokenReply("jwt-live", 900)).
				Script(testkit.LearnerRoute, testkit.JSON(testkit.LearnerBody))
			if live := env.Run("learners", "retrieve", testkit.LearnerID, "--live"); live.ExitCode != 0 {
				t.Errorf("the live profile stopped working: exit %d, %q", live.ExitCode, live.Stderr)
			}
		})
	}
}

func TestLogoutWithNothingStored(t *testing.T) {
	env := testkit.NewEnv(t)

	result := env.Run("logout", "--live")

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if want := "No credentials stored for the live profile.\n"; result.Stdout != want {
		t.Errorf("stdout = %q, want %q", result.Stdout, want)
	}
}

func TestWhoamiShowsOrganizationScopesAndEnvironment(t *testing.T) {
	tests := []struct {
		name  string
		flags []string
		mode  string
		want  string
	}{
		{
			name: "sandbox credentials",
			mode: "test",
			want: "Organization  org_acme\nScopes        learners:read, learners:write\nEnvironment   Sandbox\n",
		},
		{
			name:  "live credentials",
			flags: []string{"--live"},
			mode:  "live",
			want:  "Organization  org_acme\nScopes        learners:read, learners:write\nEnvironment   Live\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := testkit.NewEnv(t)
			loginTo(t, env, tt.flags, testkit.ClientSecret)
			env.Transport.
				Script(testkit.TokenRoute, testkit.TokenReply("jwt-cmd", 900)).
				Script(testkit.IntrospectionRoute, testkit.IntrospectionReply(tt.mode))

			result := env.Run(append([]string{"whoami"}, tt.flags...)...)

			if result.ExitCode != 0 {
				t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
			}
			if result.Stdout != tt.want {
				t.Errorf("stdout = %q, want %q", result.Stdout, tt.want)
			}
			// whoami asks every time, even with the Organization stored.
			if got := env.Transport.Routes()[2:]; strings.Join(got, ",") != testkit.TokenRoute+","+testkit.IntrospectionRoute {
				t.Errorf("routes = %v", got)
			}
		})
	}
}

// An older tmh kept a partner-login profile's Refresh token in the keyring
// under "<profile>:refresh-token". Nothing reads it any more, so logging in
// or out must not leave it behind.
func TestLoginAndLogoutRemoveAnOldRefreshToken(t *testing.T) {
	const oldRefreshToken = "tmh_rt_grt_01JD8A2B3C4D5E6F7G8H9J0K1M.old-entropy"
	tests := []struct {
		name string
		run  func(t *testing.T, env *testkit.Env)
	}{
		{name: "login", run: func(t *testing.T, env *testkit.Env) {
			loginTo(t, env, nil, testkit.ClientSecret)
		}},
		{name: "logout", run: func(t *testing.T, env *testkit.Env) {
			if logout := env.Run("logout"); logout.ExitCode != 0 {
				t.Fatalf("logout exit code = %d, stderr = %q", logout.ExitCode, logout.Stderr)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := testkit.NewEnv(t)
			if err := env.Keyring.Set("tmh", "sandbox:refresh-token", oldRefreshToken); err != nil {
				t.Fatal(err)
			}

			tt.run(t, env)

			if token, err := env.Keyring.Get("tmh", "sandbox:refresh-token"); err == nil {
				t.Errorf("the keyring still holds the old Refresh token %q", token)
			}
		})
	}
}
