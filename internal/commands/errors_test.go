package commands_test

import (
	"errors"
	"testing"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
)

func TestAPIFailuresGoToStderrWithTheRequestID(t *testing.T) {
	tests := []struct {
		name       string
		reply      testkit.Reply
		wantStderr string
		wantExit   int
	}{
		{
			name: "problem+json not found",
			reply: testkit.Reply{
				Status: 404,
				Header: map[string]string{"Content-Type": "application/problem+json", "X-Request-Id": "req-404"},
				Body:   `{"code":"not_found","title":"Not found","status":404,"detail":"No Learner with that id."}`,
			},
			wantStderr: "Error: Not found: No Learner with that id.\nRequest ID: req-404\n",
			wantExit:   3,
		},
		{
			name: "an HTML page where JSON was expected",
			reply: testkit.Reply{
				Status: 200,
				Header: map[string]string{"Content-Type": "text/html; charset=utf-8"},
				Body:   "<!doctype html><html></html>",
			},
			wantStderr: "Error: expected a JSON response but got text/html; charset=utf-8 with HTTP 200; check the base URL\n",
			wantExit:   7,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := loggedInByEnvironment(t)
			env.Transport.Script(testkit.LearnerRoute, tt.reply)

			result := env.Run("learners", "retrieve", testkit.LearnerID)

			if result.ExitCode != tt.wantExit {
				t.Errorf("exit code = %d, want %d", result.ExitCode, tt.wantExit)
			}
			if result.Stderr != tt.wantStderr {
				t.Errorf("stderr = %q, want %q", result.Stderr, tt.wantStderr)
			}
			if result.Stdout != "" {
				t.Errorf("stdout = %q, want nothing", result.Stdout)
			}
		})
	}
}

func TestNoResponseIsAConnectionFailure(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(testkit.LearnerRoute, testkit.Reply{Err: errors.New("connection refused")})

	result := env.Run("learners", "retrieve", testkit.LearnerID)

	if result.ExitCode != 7 {
		t.Errorf("exit code = %d, want 7", result.ExitCode)
	}
	want := `Error: could not reach the TeachMeHIPAA API: Get "https://api-teachmehipaa-dev.web.app/v1/organizations/org_acme/learners/` +
		testkit.LearnerID + `": connection refused` + "\n"
	if result.Stderr != want {
		t.Errorf("stderr = %q, want %q", result.Stderr, want)
	}
}
