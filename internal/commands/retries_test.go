package commands_test

import (
	"errors"
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
)

func unavailable() testkit.Reply { return problem(503, "unavailable") }

var errConnectionReset = errors.New("connection reset by peer")

func countRoute(env *testkit.Env, route string) int {
	n := 0
	for _, r := range env.Transport.Routes() {
		if r == route {
			n++
		}
	}
	return n
}

func TestTransientFailuresOfARetrieveAreRetried(t *testing.T) {
	tests := []struct {
		name  string
		reply testkit.Reply
	}{
		{"no response", testkit.Reply{Err: errors.New("connection reset by peer")}},
		{"429 rate limited", problem(429, "rate_limited")},
		{"409 conflict", problem(409, "conflict")},
		{"500 internal", problem(500, "internal")},
		{"502 as an HTML page", testkit.Reply{Status: 502, Header: map[string]string{"Content-Type": "text/html"}, Body: "<html>"}},
		{"503 unavailable", unavailable()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := loggedInByEnvironment(t)
			env.Transport.Script(testkit.LearnerRoute, tt.reply, tt.reply, testkit.JSON(testkit.LearnerBody))

			result := env.Run("learners", "retrieve", testkit.LearnerID)

			if result.ExitCode != 0 {
				t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
			}
			if want := testkit.LearnerBody + "\n"; result.Stdout != want {
				t.Errorf("stdout = %q, want %q", result.Stdout, want)
			}
			if n := countRoute(env, testkit.LearnerRoute); n != 3 {
				t.Errorf("learner requests = %d, want 3", n)
			}
		})
	}
}

func TestFailuresThatWontChangeAreNotRetried(t *testing.T) {
	tests := []struct {
		name  string
		reply testkit.Reply
	}{
		{"409 with another code", problem(409, "seat_unavailable")},
		{"400 bad request", problem(400, "bad_request")},
		{"403 insufficient scope", problem(403, "insufficient_scope")},
		{"404 not found", problem(404, "not_found")},
		{"200 with an HTML page", testkit.Reply{Status: 200, Header: map[string]string{"Content-Type": "text/html"}, Body: "<html>"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := loggedInByEnvironment(t)
			env.Transport.Script(testkit.LearnerRoute, tt.reply)

			result := env.Run("learners", "retrieve", testkit.LearnerID)

			if result.ExitCode == 0 {
				t.Fatal("exit code = 0, want a failure")
			}
			if n := countRoute(env, testkit.LearnerRoute); n != 1 {
				t.Errorf("learner requests = %d, want 1", n)
			}
			if sleeps := env.Clock.Sleeps(); len(sleeps) != 0 {
				t.Errorf("slept %v, want no waits", sleeps)
			}
		})
	}
}

func TestRetriesStopAfterMaxRetries(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantRequests int
	}{
		{name: "two by default", args: nil, wantRequests: 3},
		{name: "--max-retries 1", args: []string{"--max-retries", "1"}, wantRequests: 2},
		{name: "--max-retries 0", args: []string{"--max-retries", "0"}, wantRequests: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := loggedInByEnvironment(t)
			last := problem(503, "unavailable")
			last.Header["X-Request-Id"] = "req-last"
			replies := slices.Repeat([]testkit.Reply{unavailable()}, tt.wantRequests-1)
			env.Transport.Script(testkit.LearnerRoute, append(replies, last)...)

			result := env.Run(slices.Concat([]string{"learners", "retrieve", testkit.LearnerID}, tt.args)...)

			if result.ExitCode != 7 {
				t.Errorf("exit code = %d, want 7", result.ExitCode)
			}
			if want := "Error: Failed\nRequest ID: req-last\n"; result.Stderr != want {
				t.Errorf("stderr = %q, want %q", result.Stderr, want)
			}
			if n := countRoute(env, testkit.LearnerRoute); n != tt.wantRequests {
				t.Errorf("learner requests = %d, want %d", n, tt.wantRequests)
			}
		})
	}
}

func TestBackoffDoublesFromHalfASecondToFiveSecondsWithJitter(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(testkit.LearnerRoute, slices.Repeat([]testkit.Reply{unavailable()}, 7)...)

	env.Run("learners", "retrieve", testkit.LearnerID, "--max-retries", "6")

	// Each wait is 50–100 % of a ceiling of 0.5, 1, 2, 4, 5, 5 s, and never
	// under 0.5 s.
	bounds := [][2]time.Duration{
		{500 * time.Millisecond, 500 * time.Millisecond},
		{500 * time.Millisecond, time.Second},
		{time.Second, 2 * time.Second},
		{2 * time.Second, 4 * time.Second},
		{2500 * time.Millisecond, 5 * time.Second},
		{2500 * time.Millisecond, 5 * time.Second},
	}
	sleeps := env.Clock.Sleeps()
	if len(sleeps) != len(bounds) {
		t.Fatalf("sleeps = %v, want %d", sleeps, len(bounds))
	}
	for i, sleep := range sleeps {
		if sleep < bounds[i][0] || sleep > bounds[i][1] {
			t.Errorf("wait %d = %v, want between %v and %v", i+1, sleep, bounds[i][0], bounds[i][1])
		}
	}
}

func TestRetryAfterIsHonoredUpTo60Seconds(t *testing.T) {
	tests := []struct {
		retryAfter string
		want       time.Duration
	}{
		{retryAfter: "7", want: 7 * time.Second},
		{retryAfter: "120", want: 60 * time.Second},
		// The clock starts at 12:00:00 UTC on 2026-11-15.
		{retryAfter: "Sun, 15 Nov 2026 12:00:30 GMT", want: 30 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.retryAfter, func(t *testing.T) {
			env := loggedInByEnvironment(t)
			limited := problem(429, "rate_limited")
			limited.Header["Retry-After"] = tt.retryAfter
			env.Transport.Script(testkit.LearnerRoute, limited, testkit.JSON(testkit.LearnerBody))

			result := env.Run("learners", "retrieve", testkit.LearnerID)

			if result.ExitCode != 0 {
				t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
			}
			if sleeps := env.Clock.Sleeps(); !slices.Equal(sleeps, []time.Duration{tt.want}) {
				t.Errorf("sleeps = %v, want [%v]", sleeps, tt.want)
			}
		})
	}
}

var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestARetriedWriteReusesItsIdempotencyKey(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(testkit.RemindRoute, unavailable(), testkit.Reply{Err: errors.New("timeout")},
		testkit.JSON(testkit.RemindAcceptedBody))

	result := env.Run("enrollments", "remind", testkit.EnrollmentID)

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	keys := idempotencyKeys(env, testkit.RemindRoute)
	if len(keys) != 3 {
		t.Fatalf("remind requests = %d, want 3", len(keys))
	}
	if !uuidV4.MatchString(keys[0]) {
		t.Errorf("Idempotency-Key = %q, want a UUIDv4", keys[0])
	}
	if keys[1] != keys[0] || keys[2] != keys[0] {
		t.Errorf("Idempotency-Keys = %v, want one key on every attempt", keys)
	}
}

func TestEachCommandGeneratesItsOwnIdempotencyKey(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.
		Script(testkit.TokenRoute, testkit.TokenReply("jwt-2", 900)).
		Script(testkit.IntrospectionRoute, testkit.IntrospectionReply("test")).
		Script(testkit.RemindRoute, testkit.JSON(testkit.RemindAcceptedBody), testkit.JSON(testkit.RemindAcceptedBody))

	env.Run("enrollments", "remind", testkit.EnrollmentID)
	env.Run("enrollments", "remind", testkit.EnrollmentID)

	if keys := idempotencyKeys(env, testkit.RemindRoute); len(keys) != 2 || keys[0] == keys[1] {
		t.Errorf("Idempotency-Keys = %v, want two different keys", keys)
	}
}

func TestTheIdempotencyKeyFlagReplacesTheGeneratedKey(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(testkit.RemindRoute, unavailable(), testkit.JSON(testkit.RemindAcceptedBody))

	result := env.Run("enrollments", "remind", testkit.EnrollmentID, "--idempotency-key", "nightly-sync-2026-11-15")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if keys := idempotencyKeys(env, testkit.RemindRoute); !slices.Equal(keys, []string{"nightly-sync-2026-11-15", "nightly-sync-2026-11-15"}) {
		t.Errorf("Idempotency-Keys = %v", keys)
	}
}

func TestAWriteTheIRMarksRetrySafeIsRetriedWithoutAKey(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(testkit.LaunchLinkRoute, unavailable(), testkit.JSON(testkit.LaunchLinkBody))

	result := env.Run("learners", "create-launch-link", testkit.LearnerID)

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if keys := idempotencyKeys(env, testkit.LaunchLinkRoute); !slices.Equal(keys, []string{"", ""}) {
		t.Errorf("Idempotency-Keys = %q, want two requests without one", keys)
	}
}

func TestAnIdempotencyKeyNeverMakesADeleteRetryable(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(testkit.DeleteWebhookRoute, unavailable(), testkit.Reply{Status: 204})

	result := env.Run("webhook-subscriptions", "delete", testkit.SubscriptionID, "--idempotency-key", "cleanup-1")

	if result.ExitCode != 7 {
		t.Errorf("exit code = %d, want 7", result.ExitCode)
	}
	if keys := idempotencyKeys(env, testkit.DeleteWebhookRoute); !slices.Equal(keys, []string{"cleanup-1"}) {
		t.Errorf("Idempotency-Keys = %q, want one request carrying cleanup-1", keys)
	}
}

func TestADeleteAnsweredWith204PrintsNothing(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(testkit.DeleteWebhookRoute, testkit.Reply{Status: 204, Header: map[string]string{"Content-Type": ""}})

	result := env.Run("webhook-subscriptions", "delete", testkit.SubscriptionID)

	if result.ExitCode != 0 || result.Stdout != "" {
		t.Errorf("exit code = %d, stdout = %q, stderr = %q; want 0 and nothing printed", result.ExitCode, result.Stdout, result.Stderr)
	}
	if keys := idempotencyKeys(env, testkit.DeleteWebhookRoute); !slices.Equal(keys, []string{""}) {
		t.Errorf("Idempotency-Keys = %q, want none", keys)
	}
}

func TestA401IsReplayedOnceWithAFreshTokenWithoutUsingARetry(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.
		Script(testkit.TokenRoute, testkit.TokenReply("jwt-2", 900)).
		Script(testkit.LearnerRoute, problem(401, "unauthorized"), testkit.JSON(testkit.LearnerBody))

	result := env.Run("learners", "retrieve", testkit.LearnerID, "--max-retries", "0")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	want := []string{testkit.TokenRoute, testkit.IntrospectionRoute, testkit.LearnerRoute, testkit.TokenRoute, testkit.LearnerRoute}
	if routes := env.Transport.Routes(); !slices.Equal(routes, want) {
		t.Errorf("routes = %v, want %v", routes, want)
	}
	requests := env.Transport.Requests()
	assertHeader(t, requests[4], "Authorization", "Bearer jwt-2")
	if sleeps := env.Clock.Sleeps(); len(sleeps) != 0 {
		t.Errorf("slept %v before the replay, want no wait", sleeps)
	}
}

func TestA401ForTheFreshTokenIsNotReplayedAgain(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.
		Script(testkit.TokenRoute, testkit.TokenReply("jwt-2", 900)).
		Script(testkit.LearnerRoute, problem(401, "unauthorized"), problem(401, "unauthorized"))

	result := env.Run("learners", "retrieve", testkit.LearnerID)

	if result.ExitCode != 2 {
		t.Errorf("exit code = %d, want 2", result.ExitCode)
	}
	if n := countRoute(env, testkit.TokenRoute); n != 2 {
		t.Errorf("token requests = %d, want 2", n)
	}
}

func TestThe401ReplayHappensOncePerCommandEvenAcrossRetries(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.
		Script(testkit.TokenRoute, testkit.TokenReply("jwt-2", 900)).
		Script(testkit.LearnerRoute, problem(401, "unauthorized"), unavailable(), problem(401, "unauthorized"))

	result := env.Run("learners", "retrieve", testkit.LearnerID)

	if result.ExitCode != 2 {
		t.Errorf("exit code = %d, want 2; stderr = %q", result.ExitCode, result.Stderr)
	}
	if n := countRoute(env, testkit.TokenRoute); n != 2 {
		t.Errorf("token requests = %d, want 2", n)
	}
}

func TestAClientCredentialsMintIsRetriedOnServerAndNetworkFailures(t *testing.T) {
	env := testkit.NewEnv(t)
	env.Vars["TMH_CLIENT_ID"] = testkit.ClientID
	env.Vars["TMH_CLIENT_SECRET"] = testkit.ClientSecret
	env.Transport.
		Script(testkit.TokenRoute, testkit.Reply{Err: errors.New("connection refused")}, unavailable(), testkit.TokenReply("jwt-1", 900)).
		Script(testkit.IntrospectionRoute, testkit.IntrospectionReply("test")).
		Script(testkit.LearnerRoute, testkit.JSON(testkit.LearnerBody))

	result := env.Run("learners", "retrieve", testkit.LearnerID)

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if n := countRoute(env, testkit.TokenRoute); n != 3 {
		t.Errorf("token requests = %d, want 3", n)
	}
}

func idempotencyKeys(env *testkit.Env, route string) []string {
	keys := []string{}
	for _, r := range env.Transport.Requests() {
		if r.Method+" "+r.Path == route {
			keys = append(keys, r.Header.Get("Idempotency-Key"))
		}
	}
	return keys
}
