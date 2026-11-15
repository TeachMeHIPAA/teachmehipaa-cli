package commands_test

import (
	"strings"
	"testing"
	"time"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
)

func TestVerbosePrintsOneLinePerHTTPAttempt(t *testing.T) {
	env := testkit.NewEnv(t)
	env.Vars["TMH_CLIENT_ID"] = testkit.ClientID
	env.Vars["TMH_CLIENT_SECRET"] = testkit.ClientSecret
	token := testkit.TokenReply("jwt-1", 900)
	token.Header["X-Request-Id"] = "req-token"
	token.Before = func() { env.Clock.Advance(85 * time.Millisecond) }
	introspection := testkit.IntrospectionReply("test")
	introspection.Header = map[string]string{"X-Request-Id": "req-introspect"}
	failed := problem(503, "unavailable")
	failed.Header["X-Request-Id"] = "req-503"
	failed.Before = func() { env.Clock.Advance(2 * time.Second) }
	learner := testkit.JSON(testkit.LearnerBody)
	learner.Header = map[string]string{"X-Request-Id": "req-learner"}
	learner.Before = func() { env.Clock.Advance(120 * time.Millisecond) }
	env.Transport.
		Script(testkit.TokenRoute, token).
		Script(testkit.IntrospectionRoute, introspection).
		Script(testkit.LearnerRoute, failed, learner)

	result := env.Run("learners", "retrieve", testkit.LearnerID, "--course-id", "hipaa-2026", "--verbose")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	want := "POST /v1/auth/token 200 85ms request-id=req-token\n" +
		"GET /v1/auth/introspection 200 0ms request-id=req-introspect\n" +
		"GET /v1/organizations/org_acme/learners/" + testkit.LearnerID + " 503 2000ms request-id=req-503\n" +
		"GET /v1/organizations/org_acme/learners/" + testkit.LearnerID + " 200 120ms request-id=req-learner\n"
	if result.Stderr != want {
		t.Errorf("stderr = %q, want %q", result.Stderr, want)
	}
	if want := testkit.LearnerBody + "\n"; result.Stdout != want {
		t.Errorf("stdout = %q, want only the Learner", result.Stdout)
	}
}

func TestVerboseShowsTheIdempotencyKeyAndMissingResponses(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(testkit.RemindRoute, testkit.Reply{Err: errConnectionReset}, testkit.JSON(testkit.RemindAcceptedBody))

	result := env.Run("enrollments", "remind", testkit.EnrollmentID, "--verbose", "--idempotency-key", "k-1")

	path := "/v1/organizations/org_acme/enrollments/" + testkit.EnrollmentID + ":remind"
	for _, line := range []string{
		"POST " + path + " no response 0ms idempotency-key=k-1\n",
		"POST " + path + " 200 0ms idempotency-key=k-1\n",
	} {
		if !strings.Contains(result.Stderr, line) {
			t.Errorf("stderr lacks %q:\n%s", line, result.Stderr)
		}
	}
}

func TestVerboseNeverPrintsBodiesQueriesOrCredentials(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(testkit.LearnerRoute, testkit.JSON(testkit.LearnerBody))

	result := env.Run("learners", "retrieve", testkit.LearnerID, "--course-id", "hipaa-2026", "--verbose", "--show-headers")

	for _, leak := range []string{
		testkit.ClientSecret, "jwt-1", "Authorization", "Bearer", "Basic",
		"ana@example.com", "grant_type", "courseId", "hipaa-2026",
	} {
		if strings.Contains(result.Stderr, leak) {
			t.Errorf("stderr shows %q:\n%s", leak, result.Stderr)
		}
	}
}

func TestShowHeadersPrintsEachResponsesHeaders(t *testing.T) {
	env := loggedInByEnvironment(t)
	learner := testkit.JSON(testkit.LearnerBody)
	learner.Header = map[string]string{
		"X-Request-Id": "req-learner",
		"Set-Cookie":   "session=abc123; HttpOnly",
		"Etag":         `"v3"`,
	}
	env.Transport.Script(testkit.LearnerRoute, learner)

	result := env.Run("learners", "retrieve", testkit.LearnerID, "--show-headers")

	want := "GET /v1/organizations/org_acme/learners/" + testkit.LearnerID + " 200 0ms request-id=req-learner\n" +
		"< Content-Type: application/json\n" +
		"< Etag: \"v3\"\n" +
		"< Set-Cookie: [redacted]\n" +
		"< X-Request-Id: req-learner\n"
	if !strings.HasSuffix(result.Stderr, want) {
		t.Errorf("stderr = %q, want it to end with %q", result.Stderr, want)
	}
	if strings.Contains(result.Stderr, "abc123") {
		t.Error("stderr shows the cookie")
	}
	if want := testkit.LearnerBody + "\n"; result.Stdout != want {
		t.Errorf("stdout = %q, want only the Learner", result.Stdout)
	}
}

func TestWithoutVerboseNothingButErrorsReachesStderr(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(testkit.LearnerRoute, unavailable(), testkit.JSON(testkit.LearnerBody))

	if result := env.Run("learners", "retrieve", testkit.LearnerID); result.Stderr != "" {
		t.Errorf("stderr = %q, want nothing", result.Stderr)
	}
}
