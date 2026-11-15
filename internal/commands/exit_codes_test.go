package commands_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
)

func problem(status int, code string) testkit.Reply {
	return testkit.Reply{
		Status: status,
		Header: map[string]string{"Content-Type": "application/problem+json"},
		Body:   fmt.Sprintf(`{"code":%q,"title":"Failed","status":%d}`, code, status),
	}
}

func TestEachAPIFailureClassHasItsOwnExitCode(t *testing.T) {
	tests := []struct {
		name     string
		reply    testkit.Reply
		wantExit int
	}{
		{"401 unauthorized after the replay", problem(401, "unauthorized"), 2},
		{"403 insufficient scope", problem(403, "insufficient_scope"), 2},
		{"404 not found", problem(404, "not_found"), 3},
		{"409 seat unavailable", problem(409, "seat_unavailable"), 4},
		{"409 conflict", problem(409, "conflict"), 4},
		{"412 precondition failed", problem(412, "precondition_failed"), 4},
		{"428 precondition required", problem(428, "precondition_required"), 4},
		{"400 bad request", problem(400, "bad_request"), 5},
		{"405 method not allowed", problem(405, "method_not_allowed"), 5},
		{"406 not acceptable", problem(406, "not_acceptable"), 5},
		{"415 unsupported media type", problem(415, "unsupported_media_type"), 5},
		{"422 validation failed", problem(422, "validation_failed"), 5},
		{"422 idempotency key reuse", problem(422, "idempotency_key_reuse"), 5},
		{"429 rate limited", problem(429, "rate_limited"), 6},
		{"429 from a proxy as HTML", testkit.Reply{Status: 429, Header: map[string]string{"Content-Type": "text/html"}, Body: "<html>"}, 6},
		{"500 internal", problem(500, "internal"), 7},
		{"502 bad gateway as HTML", testkit.Reply{Status: 502, Header: map[string]string{"Content-Type": "text/html"}, Body: "<html>"}, 7},
		{"503 unavailable", problem(503, "unavailable"), 7},
		{"200 with an HTML page", testkit.Reply{Status: 200, Header: map[string]string{"Content-Type": "text/html"}, Body: "<html>"}, 7},
		{"404 with an HTML page", testkit.Reply{Status: 404, Header: map[string]string{"Content-Type": "text/html"}, Body: "<html>"}, 7},
		{"200 with no body", testkit.Reply{Status: 200, Header: map[string]string{"Content-Type": ""}}, 7},
		{"no response", testkit.Reply{Err: errors.New("connection reset by peer")}, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := loggedInByEnvironment(t)
			// A 401 `unauthorized` is replayed once with a new token first.
			env.Transport.Script(testkit.TokenRoute, testkit.TokenReply("jwt-2", 900))
			env.Transport.Script(testkit.LearnerRoute, tt.reply, tt.reply)

			result := env.Run("learners", "retrieve", testkit.LearnerID, "--max-retries=0")

			if result.ExitCode != tt.wantExit {
				t.Errorf("exit code = %d, want %d; stderr = %q", result.ExitCode, tt.wantExit, result.Stderr)
			}
			if result.Stdout != "" {
				t.Errorf("stdout = %q, want nothing", result.Stdout)
			}
		})
	}
}

func TestTokenEndpointFailuresHaveTheirOwnExitCodes(t *testing.T) {
	tests := []struct {
		name       string
		reply      testkit.Reply
		wantExit   int
		wantStderr string
	}{
		{
			name: "invalid_client",
			reply: testkit.Reply{
				Status: 401,
				Header: map[string]string{"X-Request-Id": "req-token"},
				Body:   `{"error":"invalid_client","error_description":"Client authentication failed."}`,
			},
			wantExit:   2,
			wantStderr: "Error: invalid_client: Client authentication failed.\nRequest ID: req-token\n",
		},
		{
			name:       "invalid_scope",
			reply:      testkit.Reply{Status: 400, Body: `{"error":"invalid_scope"}`},
			wantExit:   2,
			wantStderr: "Error: invalid_scope\n",
		},
		{
			name:       "an HTML page from a wrong base URL",
			reply:      testkit.Reply{Status: 404, Header: map[string]string{"Content-Type": "text/html"}, Body: "<html>"},
			wantExit:   7,
			wantStderr: "Error: expected a JSON response but got text/html with HTTP 404; check the base URL\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := testkit.NewEnv(t)
			env.Vars["TMH_CLIENT_ID"] = testkit.ClientID
			env.Vars["TMH_CLIENT_SECRET"] = testkit.ClientSecret
			env.Transport.Script(testkit.TokenRoute, tt.reply)

			result := env.Run("learners", "retrieve", testkit.LearnerID)

			if result.ExitCode != tt.wantExit {
				t.Errorf("exit code = %d, want %d", result.ExitCode, tt.wantExit)
			}
			if result.Stderr != tt.wantStderr {
				t.Errorf("stderr = %q, want %q", result.Stderr, tt.wantStderr)
			}
			if routes := env.Transport.Routes(); len(routes) != 1 {
				t.Errorf("routes = %v, want one token request", routes)
			}
		})
	}
}

func TestSuccessAndUsageExitCodes(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(testkit.LearnerRoute, testkit.JSON(testkit.LearnerBody))

	if ok := env.Run("learners", "retrieve", testkit.LearnerID); ok.ExitCode != 0 {
		t.Errorf("success exit code = %d, stderr = %q", ok.ExitCode, ok.Stderr)
	}
	for _, args := range [][]string{
		{"learners", "retrieve"},
		{"learners", "retrieve", testkit.LearnerID, "--no-such-flag"},
		{"learners", "retrieve", testkit.LearnerID, "--max-retries=-1"},
	} {
		if usage := env.Run(args...); usage.ExitCode != 1 {
			t.Errorf("tmh %v exit code = %d, want 1 (usage)", args, usage.ExitCode)
		}
	}
}
