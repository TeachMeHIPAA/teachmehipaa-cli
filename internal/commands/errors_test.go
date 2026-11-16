package commands_test

import (
	"errors"
	"strings"
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
	refused := testkit.Reply{Err: errors.New("connection refused")}
	env.Transport.Script(testkit.LearnerRoute, refused, refused, refused)

	result := env.Run("learners", "retrieve", testkit.LearnerID)

	if result.ExitCode != 7 {
		t.Errorf("exit code = %d, want 7", result.ExitCode)
	}
	want := `Error: could not reach the TeachMeHIPAA API: Get "https://sandbox.api.tme.so/v1/organizations/org_acme/learners/` +
		testkit.LearnerID + `": connection refused` + "\n"
	if result.Stderr != want {
		t.Errorf("stderr = %q, want %q", result.Stderr, want)
	}
}

func TestFieldErrorsArePrintedUnderTheTitle(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(testkit.RemindRoute, testkit.Reply{
		Status: 422,
		Header: map[string]string{"Content-Type": "application/problem+json", "X-Request-Id": "req-422"},
		Body: `{"code":"validation_failed","title":"Validation failed","status":422,"detail":"The request has invalid fields.",` +
			`"errors":[{"path":"Idempotency-Key","message":"Required"},{"path":"learner.email","message":"Invalid email"}]}`,
	})

	result := env.Run("enrollments", "remind", testkit.EnrollmentID)

	if result.ExitCode != 5 {
		t.Errorf("exit code = %d, want 5", result.ExitCode)
	}
	want := "Error: Validation failed: The request has invalid fields.\n" +
		"  Idempotency-Key: Required\n" +
		"  learner.email: Invalid email\n" +
		"Request ID: req-422\n"
	if result.Stderr != want {
		t.Errorf("stderr = %q, want %q", result.Stderr, want)
	}
}

func TestNoErrorShowsTheClientSecretOrAccessToken(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(testkit.LearnerRoute, problem(401, "unauthorized"))
	env.Transport.Script(testkit.TokenRoute, testkit.Reply{Status: 401, Body: `{"error":"invalid_client"}`})

	result := env.Run("learners", "retrieve", testkit.LearnerID, "--verbose", "--show-headers")

	if result.ExitCode != 2 {
		t.Errorf("exit code = %d, want 2", result.ExitCode)
	}
	for _, secret := range []string{testkit.ClientSecret, "jwt-1", "Bearer", "Basic"} {
		if strings.Contains(result.Stderr, secret) {
			t.Errorf("stderr shows %q:\n%s", secret, result.Stderr)
		}
	}
}
