package commands_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/version"
)

const latestReleaseRoute = "GET /repos/TeachMeHIPAA/teachmehipaa-cli/releases/latest"

func latestRelease(tag string) testkit.Reply {
	return testkit.JSON(`{"tag_name":"` + tag + `","name":"` + tag + `","draft":false,"prerelease":false}`)
}

// onATerminal is a person's shell on a release build, where the notice runs.
func onATerminal(t *testing.T) *testkit.Env {
	t.Helper()
	env := testkit.NewEnv(t)
	env.StderrIsTTY = true
	env.Build = version.Info{Version: "1.2.0", Commit: "abc1234", Date: "2026-11-01T00:00:00Z"}
	return env
}

func TestANewerReleaseIsAnnouncedOnStderr(t *testing.T) {
	env := onATerminal(t)
	env.UpdateTransport.Script(latestReleaseRoute, latestRelease("v1.3.0"))

	result := env.Run("version")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if result.Stdout != "tmh version 1.2.0 (commit abc1234, built 2026-11-01T00:00:00Z)\n" {
		t.Errorf("stdout = %q, want only the version", result.Stdout)
	}
	want := "A new version of tmh is available: v1.3.0 (you have v1.2.0). " +
		"https://github.com/TeachMeHIPAA/teachmehipaa-cli/releases/tag/v1.3.0\n"
	if result.Stderr != want {
		t.Errorf("stderr = %q, want %q", result.Stderr, want)
	}
	request := env.UpdateTransport.Requests()[0]
	if request.URL != "https://api.github.com/repos/TeachMeHIPAA/teachmehipaa-cli/releases/latest" {
		t.Errorf("URL = %s", request.URL)
	}
	assertHeader(t, request, "Authorization", "")
}

func TestNoNoticeUnlessTheReleaseIsNewer(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
	}{
		{"same version", "1.2.0", "v1.2.0"},
		{"older release", "1.10.0", "v1.9.9"},
		{"newer prerelease build", "1.3.0-rc.1", "v1.2.9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := onATerminal(t)
			env.Build.Version = tt.current
			env.UpdateTransport.Script(latestReleaseRoute, latestRelease(tt.latest))

			result := env.Run("version")

			if result.Stderr != "" {
				t.Errorf("stderr = %q, want no notice", result.Stderr)
			}
			if n := len(env.UpdateTransport.Routes()); n != 1 {
				t.Errorf("release checks = %d, want 1", n)
			}
		})
	}
}

func TestAReleaseOfAPrereleaseBuildIsNewer(t *testing.T) {
	env := onATerminal(t)
	env.Build.Version = "1.3.0-rc.1"
	env.UpdateTransport.Script(latestReleaseRoute, latestRelease("v1.3.0"))

	result := env.Run("version")

	want := "A new version of tmh is available: v1.3.0 (you have v1.3.0-rc.1). " +
		"https://github.com/TeachMeHIPAA/teachmehipaa-cli/releases/tag/v1.3.0\n"
	if result.Stderr != want {
		t.Errorf("stderr = %q, want %q", result.Stderr, want)
	}
}

func TestTheReleaseCheckRunsAtMostOnceADay(t *testing.T) {
	env := onATerminal(t)
	env.UpdateTransport.Script(latestReleaseRoute, latestRelease("v1.3.0"), latestRelease("v1.3.0"))

	first := env.Run("version")
	env.Clock.Advance(24*time.Hour - time.Second)
	second := env.Run("version")
	env.Clock.Advance(time.Second)
	third := env.Run("version")

	if n := len(env.UpdateTransport.Routes()); n != 2 {
		t.Errorf("release checks = %d, want 2 (first run and 24 h later)", n)
	}
	if first.Stderr == "" || second.Stderr != "" || third.Stderr == "" {
		t.Errorf("stderr = %q, %q, %q; want a notice only when checked", first.Stderr, second.Stderr, third.Stderr)
	}
}

func TestTheReleaseCheckIsOffWhereNobodyWouldSeeIt(t *testing.T) {
	tests := []struct {
		name  string
		setup func(env *testkit.Env)
	}{
		{"TMH_NO_UPDATE_CHECK=1", func(env *testkit.Env) { env.Vars["TMH_NO_UPDATE_CHECK"] = "1" }},
		{"CI", func(env *testkit.Env) { env.Vars["CI"] = "true" }},
		{"stderr is not a terminal", func(env *testkit.Env) { env.StderrIsTTY = false }},
		{"a development build", func(env *testkit.Env) { env.Build.Version = "dev" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := onATerminal(t)
			tt.setup(env)

			result := env.Run("version")

			if result.ExitCode != 0 || result.Stderr != "" {
				t.Errorf("exit code = %d, stderr = %q; want 0 and nothing", result.ExitCode, result.Stderr)
			}
			if routes := slices.Concat(env.Transport.Routes(), env.UpdateTransport.Routes()); len(routes) != 0 {
				t.Errorf("sent %v, want no requests", routes)
			}
		})
	}
}

func TestCompletionNeverChecksForReleases(t *testing.T) {
	for _, args := range [][]string{{"completion", "zsh"}, {"__complete", "open", ""}} {
		t.Run(args[0], func(t *testing.T) {
			env := onATerminal(t)

			env.Run(args...)

			if routes := slices.Concat(env.Transport.Routes(), env.UpdateTransport.Routes()); len(routes) != 0 {
				t.Errorf("sent %v, want no requests", routes)
			}
		})
	}
}

func TestAFailedReleaseCheckNeverFailsTheCommand(t *testing.T) {
	tests := []struct {
		name  string
		reply testkit.Reply
	}{
		{"no response", testkit.Reply{Err: errors.New("dial tcp: lookup api.github.com: no such host")}},
		{"rate limited", testkit.Reply{Status: 403, Body: `{"message":"API rate limit exceeded"}`}},
		{"not JSON", testkit.Reply{Status: 200, Header: map[string]string{"Content-Type": "text/html"}, Body: "<html>"}},
		{"not a version", latestRelease("nightly")},
		{"a terminal escape", latestRelease(`v9.9.9\u001b]8;;https://evil.example\u0007`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := onATerminal(t)
			env.UpdateTransport.Script(latestReleaseRoute, tt.reply)

			result := env.Run("version")

			if result.ExitCode != 0 || result.Stderr != "" {
				t.Errorf("exit code = %d, stderr = %q; want 0 and nothing", result.ExitCode, result.Stderr)
			}
		})
	}
}

func TestASlowReleaseCheckHoldsTheCommandAboutASecondAtMost(t *testing.T) {
	env := onATerminal(t)
	env.UpdateTransport.Script(latestReleaseRoute, testkit.Reply{Hang: true})

	started := time.Now()
	result := env.Run("version")
	elapsed := time.Since(started)

	if result.ExitCode != 0 || result.Stderr != "" {
		t.Errorf("exit code = %d, stderr = %q; want 0 and nothing", result.ExitCode, result.Stderr)
	}
	if elapsed > 1500*time.Millisecond {
		t.Errorf("tmh version took %v, want about 1 s at most", elapsed)
	}
}

func TestTheReleaseNoticeFollowsAFailedCommand(t *testing.T) {
	env := onATerminal(t)
	env.Vars["TMH_CLIENT_ID"] = testkit.ClientID
	env.Vars["TMH_CLIENT_SECRET"] = testkit.ClientSecret
	env.UpdateTransport.Script(latestReleaseRoute, latestRelease("v1.3.0"))
	env.Transport.
		Script(testkit.TokenRoute, testkit.TokenReply("jwt-1", 900)).
		Script(testkit.IntrospectionRoute, testkit.IntrospectionReply("test")).
		Script(testkit.LearnerRoute, problem(404, "not_found"))

	result := env.Run("learners", "retrieve", testkit.LearnerID)

	if result.ExitCode != 3 {
		t.Errorf("exit code = %d, want 3", result.ExitCode)
	}
	want := "Error: Failed\n" +
		"A new version of tmh is available: v1.3.0 (you have v1.2.0). " +
		"https://github.com/TeachMeHIPAA/teachmehipaa-cli/releases/tag/v1.3.0\n"
	if result.Stderr != want {
		t.Errorf("stderr = %q, want %q", result.Stderr, want)
	}
}

func TestTmhTalksOnlyToTheAPIAndTheReleaseCheck(t *testing.T) {
	env := onATerminal(t)
	env.Vars["TMH_CLIENT_ID"] = testkit.ClientID
	env.Vars["TMH_CLIENT_SECRET"] = testkit.ClientSecret
	env.UpdateTransport.Script(latestReleaseRoute, latestRelease("v1.2.0"))
	env.Transport.
		Script(testkit.TokenRoute, testkit.TokenReply("jwt-1", 900)).
		Script(testkit.IntrospectionRoute, testkit.IntrospectionReply("test")).
		Script(testkit.LearnerRoute, testkit.JSON(testkit.LearnerBody))

	for _, args := range [][]string{
		{"get", "/organizations/{orgId}/learners/" + testkit.LearnerID},
		{"version"},
		{"open", "docs"},
		{"completion", "bash"},
		{"whoami", "--help"},
	} {
		if result := env.Run(args...); result.ExitCode != 0 {
			t.Fatalf("tmh %v: exit code = %d, stderr = %q", args, result.ExitCode, result.Stderr)
		}
	}

	for _, r := range env.Transport.Requests() {
		if host := strings.SplitN(strings.TrimPrefix(r.URL, "https://"), "/", 2)[0]; host != "sandbox.api.tme.so" {
			t.Errorf("API transport carried %s, want only the API", r.URL)
		}
	}
	for _, r := range env.UpdateTransport.Requests() {
		if r.URL != "https://api.github.com/repos/TeachMeHIPAA/teachmehipaa-cli/releases/latest" {
			t.Errorf("version check transport carried %s, want only the latest-release lookup", r.URL)
		}
	}
	if n := len(env.UpdateTransport.Routes()); n != 1 {
		t.Errorf("release checks = %d, want 1", n)
	}
}
