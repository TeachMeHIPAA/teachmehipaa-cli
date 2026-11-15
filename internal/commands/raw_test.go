package commands_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
)

const learnersListRoute = "GET /v1/organizations/" + testkit.OrgID + "/learners"

func TestRawGetFillsTheOrgIDFromTheBoundOrganization(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(learnersListRoute, testkit.JSON(`{"items":[],"nextCursor":null}`))

	result := env.Run("get", "/organizations/{orgId}/learners", "--query", "limit=5", "--query", "status=in_progress")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if want := `{"items":[],"nextCursor":null}` + "\n"; result.Stdout != want {
		t.Errorf("stdout = %q, want %q", result.Stdout, want)
	}
	requests := env.Transport.Requests()
	learners := requests[len(requests)-1]
	if want := "https://api-teachmehipaa-dev.web.app/v1/organizations/org_acme/learners?limit=5&status=in_progress"; learners.URL != want {
		t.Errorf("URL = %s, want %s", learners.URL, want)
	}
	assertHeader(t, learners, "Authorization", "Bearer jwt-1")
	assertHeader(t, learners, "Content-Type", "")
	if learners.Body != "" {
		t.Errorf("body = %q, want none", learners.Body)
	}
}

const webhookSubscriptionsRoute = "POST /v1/organizations/" + testkit.OrgID + "/webhook-subscriptions"

func TestRawPostBuildsAJSONBodyFromDataFields(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(webhookSubscriptionsRoute, testkit.Reply{Status: 201, Body: `{"subscriptionId":"` + testkit.SubscriptionID + `"}`})

	result := env.Run("post", "/organizations/{orgId}/webhook-subscriptions",
		"-d", "url=https://example.com/webhooks",
		"-d", "eventTypes[]=learner.certified",
		"--data", "eventTypes[]=learner.expired",
		"-d", "filter.course.id=hipaa-2026",
		"-d", "filter.course.title=a=b, c",
		"-d", "enabled:=true",
		"-d", "retries:=3",
	)

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	request := env.Transport.Requests()[2]
	want := `{"enabled":true,"eventTypes":["learner.certified","learner.expired"],` +
		`"filter":{"course":{"id":"hipaa-2026","title":"a=b, c"}},"retries":3,"url":"https://example.com/webhooks"}`
	if request.Body != want {
		t.Errorf("body = %s\nwant   %s", request.Body, want)
	}
	assertHeader(t, request, "Content-Type", "application/json")
	if !uuidV4.MatchString(request.Header.Get("Idempotency-Key")) {
		t.Errorf("Idempotency-Key = %q, want a generated UUIDv4", request.Header.Get("Idempotency-Key"))
	}
}

const learnerPatchRoute = "PATCH /v1/organizations/" + testkit.OrgID + "/learners/" + testkit.LearnerID

func TestRawDataFromAFileIsSentAsWritten(t *testing.T) {
	env := loggedInByEnvironment(t)
	file := filepath.Join(t.TempDir(), "subscription.json")
	contents := "{\n  \"url\": \"https://example.com/webhooks\",\n  \"eventTypes\": [\"learner.certified\"]\n}\n"
	if err := os.WriteFile(file, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	env.Transport.Script(webhookSubscriptionsRoute, testkit.Reply{Status: 201, Body: `{}`})

	result := env.Run("post", "/v1/organizations/{orgId}/webhook-subscriptions", "--data", "@"+file)

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if got := env.Transport.Requests()[2].Body; got != contents {
		t.Errorf("body = %q, want the file unchanged", got)
	}
}

func TestRawDataFieldsOverrideTheDataDocument(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(webhookSubscriptionsRoute, testkit.Reply{Status: 201, Body: `{}`})

	result := env.Run("post", "/organizations/{orgId}/webhook-subscriptions",
		"--data", `{"url":"https://old.example.com","maxAttempts":12345678901234567890,"eventTypes":["learner.certified"]}`,
		"-d", "url=https://example.com/<hooks>&x",
		"-d", "eventTypes[]=learner.expired")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	want := `{"eventTypes":["learner.certified","learner.expired"],"maxAttempts":12345678901234567890,"url":"https://example.com/<hooks>&x"}`
	if got := env.Transport.Requests()[2].Body; got != want {
		t.Errorf("body = %s\nwant   %s", got, want)
	}
}

func TestRawDataFromStdin(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Stdin = `{"url":"https://example.com/webhooks"}`
	env.Transport.Script(webhookSubscriptionsRoute, testkit.Reply{Status: 201, Body: `{}`})

	result := env.Run("post", "/organizations/{orgId}/webhook-subscriptions", "-d", "@-")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if got := env.Transport.Requests()[2].Body; got != env.Stdin {
		t.Errorf("body = %q, want stdin", got)
	}
}

func TestRawPatchSendsAMergePatchWithIfMatch(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(learnerPatchRoute, testkit.JSON(testkit.LearnerBody))

	result := env.Run("patch", "/organizations/{orgId}/learners/"+testkit.LearnerID,
		"-d", "name=Ana Example", "-d", "externalId:=null", "--if-match", `"v7"`)

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	request := env.Transport.Requests()[2]
	if want := `{"externalId":null,"name":"Ana Example"}`; request.Body != want {
		t.Errorf("body = %s, want %s", request.Body, want)
	}
	assertHeader(t, request, "Content-Type", "application/merge-patch+json")
	assertHeader(t, request, "If-Match", `"v7"`)
	assertHeader(t, request, "Idempotency-Key", "")
}

func TestRawContentTypeSendsAnyBodyAsIs(t *testing.T) {
	env := loggedInByEnvironment(t)
	file := filepath.Join(t.TempDir(), "roster.csv")
	contents := "email,name\nana@example.com,Ana Example\n"
	if err := os.WriteFile(file, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	route := "POST /v1/organizations/" + testkit.OrgID + "/enrollments:import"
	env.Transport.Script(route, testkit.Reply{Status: 202, Body: `{}`})

	result := env.Run("post", "/organizations/{orgId}/enrollments:import", "--data", "@"+file, "--content-type", "text/csv")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	request := env.Transport.Requests()[2]
	if request.Body != contents {
		t.Errorf("body = %q, want the file unchanged", request.Body)
	}
	assertHeader(t, request, "Content-Type", "text/csv")
}

func TestRawDataMistakesAreUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"not key=value", []string{"post", "/learners", "-d", "email"}},
		{"invalid JSON document", []string{"post", "/learners", "--data", "{email:"}},
		{"invalid JSON after :=", []string{"post", "/learners", "-d", "count:=three"}},
		{"two documents", []string{"post", "/learners", "--data", "{}", "--data", "{}"}},
		{"fields onto an array", []string{"post", "/learners", "--data", "[]", "-d", "a=b"}},
		{"field below a string", []string{"post", "/learners", "-d", "a=b", "-d", "a.c=d"}},
		{"fields as CSV", []string{"post", "/learners", "-d", "a=b", "--content-type", "text/csv"}},
		{"missing file", []string{"post", "/learners", "--data", "@/nonexistent/body.json"}},
		{"a GET with a body", []string{"get", "/learners", "-d", "a=b"}},
		{"a full URL", []string{"get", "https://example.com/v1/learners"}},
		{"bad --query", []string{"get", "/learners", "--query", "limit"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := testkit.NewEnv(t)
			env.Vars["TMH_CLIENT_ID"] = testkit.ClientID
			env.Vars["TMH_CLIENT_SECRET"] = testkit.ClientSecret

			result := env.Run(tt.args...)

			if result.ExitCode != 1 {
				t.Errorf("exit code = %d, want 1; stderr = %q", result.ExitCode, result.Stderr)
			}
			if routes := env.Transport.Routes(); len(routes) != 0 {
				t.Errorf("sent %v, want no requests", routes)
			}
		})
	}
}

func TestRawWritesAreRetriedOnlyWhenTheyCarryAKeyThatCounts(t *testing.T) {
	const upsertPath = "/organizations/{orgId}/learners/by-external-id/hr-123"
	const upsertRoute = "PUT /v1/organizations/" + testkit.OrgID + "/learners/by-external-id/hr-123"
	tests := []struct {
		name         string
		args         []string
		route        string
		wantRequests int
	}{
		{"POST with its generated key", []string{"post", "/organizations/{orgId}/webhook-subscriptions"}, webhookSubscriptionsRoute, 2},
		{"PUT without a key", []string{"put", upsertPath, "-d", "email=ana@example.com"}, upsertRoute, 1},
		{"PUT with --idempotency-key", []string{"put", upsertPath, "-d", "email=ana@example.com", "--idempotency-key", "sync-1"}, upsertRoute, 2},
		{"PATCH with --idempotency-key", []string{"patch", "/organizations/{orgId}/learners/" + testkit.LearnerID, "-d", "name=Ana", "--idempotency-key", "sync-1"}, learnerPatchRoute, 1},
		{"DELETE with --idempotency-key", []string{"delete", "/organizations/{orgId}/webhook-subscriptions/" + testkit.SubscriptionID, "--idempotency-key", "sync-1"}, testkit.DeleteWebhookSubscriptionRoute, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := loggedInByEnvironment(t)
			env.Transport.Script(tt.route, unavailable(), testkit.JSON(`{}`))

			result := env.Run(tt.args...)

			keys := idempotencyKeys(env, tt.route)
			if len(keys) != tt.wantRequests {
				t.Fatalf("requests = %d, want %d; stderr = %q", len(keys), tt.wantRequests, result.Stderr)
			}
			if tt.wantRequests == 2 && (result.ExitCode != 0 || keys[0] == "" || keys[1] != keys[0]) {
				t.Errorf("exit code = %d, keys = %q; want success with one key on both attempts", result.ExitCode, keys)
			}
			if tt.wantRequests == 1 && result.ExitCode != 7 {
				t.Errorf("exit code = %d, want 7", result.ExitCode)
			}
		})
	}
}

func TestRawFailuresRenderLikeResourceCommands(t *testing.T) {
	env := loggedInByEnvironment(t)
	notFound := problem(404, "not_found")
	notFound.Header["X-Request-Id"] = "req-404"
	env.Transport.Script("GET /v1/organizations/"+testkit.OrgID+"/learners/sub_missing", notFound)

	result := env.Run("get", "organizations/{orgId}/learners/sub_missing")

	if result.ExitCode != 3 {
		t.Errorf("exit code = %d, want 3", result.ExitCode)
	}
	if want := "Error: Failed\nRequest ID: req-404\n"; result.Stderr != want {
		t.Errorf("stderr = %q, want %q", result.Stderr, want)
	}
	if result.Stdout != "" {
		t.Errorf("stdout = %q, want nothing", result.Stdout)
	}
}

func TestRawPathWithoutOrgIDSkipsIntrospection(t *testing.T) {
	env := testkit.NewEnv(t)
	env.Vars["TMH_CLIENT_ID"] = testkit.ClientID
	env.Vars["TMH_CLIENT_SECRET"] = testkit.ClientSecret
	env.Transport.
		Script(testkit.TokenRoute, testkit.TokenReply("jwt-1", 900)).
		Script("GET /v1/partners/ptn_01JD8P", testkit.JSON(`{"partnerId":"ptn_01JD8P"}`))

	result := env.Run("get", "/partners/ptn_01JD8P?expand=organizations")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if got := env.Transport.Requests()[1].URL; got != "https://api-teachmehipaa-dev.web.app/v1/partners/ptn_01JD8P?expand=organizations" {
		t.Errorf("URL = %s", got)
	}
}

func TestRawRequestsToPublicOperationsNeedNoLogin(t *testing.T) {
	tests := []struct {
		path  string
		route string
	}{
		{"/courses", "GET /v1/courses"},
		{"/v1/courses/hipaa-2026", "GET /v1/courses/hipaa-2026"},
		{"/certificate-verifications/ABC-123?expand=course", "GET /v1/certificate-verifications/ABC-123"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			env := testkit.NewEnv(t)
			env.Transport.Script(tt.route, testkit.JSON(`{}`))

			result := env.Run("get", tt.path)

			if result.ExitCode != 0 {
				t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
			}
			if routes := env.Transport.Routes(); len(routes) != 1 || routes[0] != tt.route {
				t.Fatalf("routes = %v, want only %s", routes, tt.route)
			}
			assertHeader(t, env.Transport.Requests()[0], "Authorization", "")
		})
	}
}

func TestRawRequestsToOtherPathsStillNeedALogin(t *testing.T) {
	for _, args := range [][]string{
		{"post", "/courses"},
		{"get", "/courses/hipaa-2026/learners"},
		{"get", "/partners/ptn_01JD8P"},
	} {
		env := testkit.NewEnv(t)

		result := env.Run(args...)

		if result.ExitCode != 2 {
			t.Errorf("tmh %v: exit code = %d, want 2; stderr = %q", args, result.ExitCode, result.Stderr)
		}
		if routes := env.Transport.Routes(); len(routes) != 0 {
			t.Errorf("tmh %v: sent %v, want no requests", args, routes)
		}
	}
}

func TestRawNoAuthSendsNoToken(t *testing.T) {
	env := testkit.NewEnv(t)
	env.Vars["TMH_CLIENT_ID"] = testkit.ClientID
	env.Vars["TMH_CLIENT_SECRET"] = testkit.ClientSecret
	env.Transport.Script("GET /v1/partners/ptn_01JD8P", problem(401, "unauthorized"))

	result := env.Run("get", "/partners/ptn_01JD8P", "--no-auth")

	if result.ExitCode != 2 {
		t.Errorf("exit code = %d, want 2; stderr = %q", result.ExitCode, result.Stderr)
	}
	if routes := env.Transport.Routes(); len(routes) != 1 || routes[0] != "GET /v1/partners/ptn_01JD8P" {
		t.Fatalf("routes = %v, want one unauthenticated request and no token mint", routes)
	}
	assertHeader(t, env.Transport.Requests()[0], "Authorization", "")
}

func TestRawNoAuthCantFillTheOrgID(t *testing.T) {
	env := testkit.NewEnv(t)

	result := env.Run("get", "/organizations/{orgId}/learners", "--no-auth")

	if result.ExitCode != 1 || !strings.Contains(result.Stderr, "{orgId}") {
		t.Errorf("exit code = %d, stderr = %q; want 1 naming {orgId}", result.ExitCode, result.Stderr)
	}
	if routes := env.Transport.Routes(); len(routes) != 0 {
		t.Errorf("sent %v, want no requests", routes)
	}
}
