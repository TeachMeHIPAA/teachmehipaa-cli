package commands_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
)

const (
	sandboxBase   = "https://sandbox.api.tme.so/v1"
	learnersRoute = "POST /v1/organizations/" + testkit.OrgID + "/learners"
	batchRoute    = "POST /v1/organizations/" + testkit.OrgID + "/enrollments:batch"
)

func TestAListCommandTakesQueryParametersAsFlags(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(learnersListRoute, testkit.JSON(`{"items":[],"nextCursor":null}`))

	result := env.Run("learners", "list", "--limit", "5", "--status", "in_progress", "--course-id", "hipaa-2026")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	request := env.Transport.Requests()[2]
	if want := sandboxBase + "/organizations/org_acme/learners?courseId=hipaa-2026&limit=5&status=in_progress"; request.URL != want {
		t.Errorf("URL = %s\nwant  %s", request.URL, want)
	}
	if request.Body != "" {
		t.Errorf("body = %q, want none", request.Body)
	}
}

func TestBodyFieldsAreTypedFlags(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(webhookSubscriptionsRoute, testkit.Reply{Status: 201, Body: `{"subscriptionId":"` + testkit.SubscriptionID + `"}`})

	result := env.Run("webhook-subscriptions", "create", "--endpoint-url", "https://hooks.example.com/tmh",
		"--event-types", "learner.certified,learner.expired", "--seats-low-threshold", "3")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	request := env.Transport.Requests()[2]
	want := `{"endpointUrl":"https://hooks.example.com/tmh","eventTypes":["learner.certified","learner.expired"],"seatsLowThreshold":3}`
	if request.Body != want {
		t.Errorf("body = %s\nwant   %s", request.Body, want)
	}
	assertHeader(t, request, "Content-Type", "application/json")
	if !uuidV4.MatchString(request.Header.Get("Idempotency-Key")) {
		t.Errorf("Idempotency-Key = %q, want a generated UUIDv4", request.Header.Get("Idempotency-Key"))
	}
	if want := `{"subscriptionId":"` + testkit.SubscriptionID + `"}` + "\n"; result.Stdout != want {
		t.Errorf("stdout = %q, want %q", result.Stdout, want)
	}
}

func TestFlagsOverrideDataFieldsWhichOverrideTheDataDocument(t *testing.T) {
	document := filepath.Join(t.TempDir(), "subscription.json")
	if err := os.WriteFile(document, []byte(`{"endpointUrl":"https://old.example.com","description":"From file","seatsLowThreshold":5}`), 0o600); err != nil {
		t.Fatal(err)
	}
	env := loggedInByEnvironment(t)
	env.Transport.Script(webhookSubscriptionsRoute, testkit.Reply{Status: 201, Body: `{}`})

	result := env.Run("webhook-subscriptions", "create", "--data", "@"+document,
		"-d", "description=From -d", "-d", "endpointUrl=https://data.example.com",
		"--endpoint-url", "https://hooks.example.com/tmh", "--event-types", "learner.certified,learner.expired")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	want := `{"description":"From -d","endpointUrl":"https://hooks.example.com/tmh",` +
		`"eventTypes":["learner.certified","learner.expired"],"seatsLowThreshold":5}`
	if got := env.Transport.Requests()[2].Body; got != want {
		t.Errorf("body = %s\nwant   %s", got, want)
	}
}

func TestAWriteThatRequiresAKeyReusesItsGeneratedKeyOnRetry(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(learnersRoute, unavailable(), testkit.Reply{Status: 201, Body: `{"created":true}`})

	result := env.Run("learners", "create", "--email", "ana@example.com", "--delivery", "none")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	keys := idempotencyKeys(env, learnersRoute)
	if len(keys) != 2 || !uuidV4.MatchString(keys[0]) || keys[1] != keys[0] {
		t.Errorf("Idempotency-Keys = %q, want one UUIDv4 on both attempts", keys)
	}
	if got, want := env.Transport.Requests()[3].Body, `{"delivery":"none","email":"ana@example.com"}`; got != want {
		t.Errorf("retried body = %s, want %s", got, want)
	}
}

func TestAnUpdateSendsAMergePatchWithIfMatchAndIsNeverRetried(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(learnerPatchRoute, unavailable())

	result := env.Run("learners", "update", testkit.LearnerID, "--name", "Ana Example",
		"-d", "externalId:=null", "--if-match", `"v7"`)

	if result.ExitCode != 7 {
		t.Errorf("exit code = %d, want 7; stderr = %q", result.ExitCode, result.Stderr)
	}
	if n := countRoute(env, learnerPatchRoute); n != 1 {
		t.Fatalf("PATCH requests = %d, want 1", n)
	}
	request := env.Transport.Requests()[2]
	if want := `{"externalId":null,"name":"Ana Example"}`; request.Body != want {
		t.Errorf("body = %s, want %s", request.Body, want)
	}
	assertHeader(t, request, "Content-Type", "application/merge-patch+json")
	assertHeader(t, request, "If-Match", `"v7"`)
	assertHeader(t, request, "Idempotency-Key", "")
}

func TestAnUpdateWithoutIfMatchIsAUsageError(t *testing.T) {
	env := loggedInByEnvironment(t)

	result := env.Run("learners", "update", testkit.LearnerID, "--name", "Ana Example")

	if result.ExitCode != 1 {
		t.Errorf("exit code = %d, want 1", result.ExitCode)
	}
	if !strings.Contains(result.Stderr, "if-match") {
		t.Errorf("stderr = %q, want it to name --if-match", result.Stderr)
	}
	if routes := env.Transport.Routes(); len(routes) != 0 {
		t.Errorf("sent %v, want no requests", routes)
	}
}

func TestACommandWithoutAnOrganizationSkipsIntrospectionWhenLoggedIn(t *testing.T) {
	env := testkit.NewEnv(t)
	env.Vars["TMH_CLIENT_ID"] = testkit.ClientID
	env.Vars["TMH_CLIENT_SECRET"] = testkit.ClientSecret
	route := "GET /v1/certificate-verifications/tme_7Qx2"
	env.Transport.Script(route, testkit.JSON(`{"valid":true}`))

	result := env.Run("certificate-verifications", "retrieve", "tme_7Qx2")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if routes := env.Transport.Routes(); !slices.Equal(routes, []string{route}) {
		t.Fatalf("routes = %v, want only the verification", routes)
	}
	if want := `{"valid":true}` + "\n"; result.Stdout != want {
		t.Errorf("stdout = %q, want %q", result.Stdout, want)
	}
}

func TestAPublicCommandSendsNoAuthorizationAndNeedsNoLogin(t *testing.T) {
	env := testkit.NewEnv(t)
	env.Transport.Script("GET /v1/courses/hipaa-2026", testkit.JSON(`{"courseId":"hipaa-2026"}`))

	result := env.Run("courses", "retrieve", "hipaa-2026")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	requests := env.Transport.Requests()
	if len(requests) != 1 {
		t.Fatalf("routes = %v, want only the course", env.Transport.Routes())
	}
	if got := requests[0].Header.Get("Authorization"); got != "" {
		t.Errorf("Authorization = %q, want none", got)
	}
}

func TestADeleteAnsweredWith204PrintsNothingAndSucceeds(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(testkit.DeleteWebhookSubscriptionRoute, testkit.Reply{Status: 204})

	result := env.Run("webhook-subscriptions", "delete", testkit.SubscriptionID)

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if result.Stdout != "" {
		t.Errorf("stdout = %q, want nothing", result.Stdout)
	}
}

func TestHelpShowsTheAPIDescriptionAndScopes(t *testing.T) {
	tests := []struct {
		args []string
		want []string
	}{
		{
			[]string{"webhook-subscriptions", "delete", "--help"},
			[]string{"tmh webhook-subscriptions delete <subscriptionId>", "Scopes: org:admin or webhooks:manage"},
		},
		{
			[]string{"learners", "update", "--help"},
			[]string{"Updates a learner's `name` or `externalId`", "Clear externalId with -d externalId:=null.",
				"Scopes: learners:write", "--if-match", "--name", "--course-id"},
		},
		{
			[]string{"courses", "list", "--help"},
			[]string{"Scopes: none (public, no login needed)", "--limit", "--cursor"},
		},
		{
			[]string{"learners", "batch-upsert", "--help"},
			[]string{"Set -d items:=<json>, or send the whole body with --data @file.json."},
		},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			env := testkit.NewEnv(t)

			result := env.Run(tt.args...)

			if result.ExitCode != 0 {
				t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
			}
			for _, want := range tt.want {
				if !strings.Contains(result.Stdout, want) {
					t.Errorf("help does not contain %q:\n%s", want, result.Stdout)
				}
			}
		})
	}
}

func TestTheBatchSplitsTakeOnlyTheirOwnSelection(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(batchRoute, testkit.JSON(`{}`))

	byID := env.Run("enrollments", "batch", "--operation", "remind",
		"--select.enrollment-ids", testkit.EnrollmentID+",enr_2")
	byFilterWithIDs := env.Run("enrollments", "batch-by-filter", "--operation", "remind",
		"--select.enrollment-ids", testkit.EnrollmentID)

	if byID.ExitCode != 0 {
		t.Fatalf("batch exit code = %d, stderr = %q", byID.ExitCode, byID.Stderr)
	}
	want := `{"operation":"remind","select":{"enrollmentIds":["` + testkit.EnrollmentID + `","enr_2"]}}`
	if got := env.Transport.Requests()[2].Body; got != want {
		t.Errorf("body = %s\nwant   %s", got, want)
	}
	if byFilterWithIDs.ExitCode != 1 || !strings.Contains(byFilterWithIDs.Stderr, "select.enrollment-ids") {
		t.Errorf("batch-by-filter --select.enrollment-ids: exit %d, stderr %q; want an unknown flag", byFilterWithIDs.ExitCode, byFilterWithIDs.Stderr)
	}
}

func TestTheFilterBatchSendsNestedFilterFlags(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(batchRoute, testkit.Reply{Status: 202, Body: `{}`})

	result := env.Run("enrollments", "batch-by-filter", "--operation", "remind",
		"--select.filter.status", "INVITED", "--select.filter.archived=false")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	want := `{"operation":"remind","select":{"filter":{"archived":false,"status":"INVITED"}}}`
	if got := env.Transport.Requests()[2].Body; got != want {
		t.Errorf("body = %s\nwant   %s", got, want)
	}
}

func TestDownloadsAndUploadsAskForTheirFileFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"download without --output", []string{"certificates", "download-pdf", "cert_1"}, "-o/--output"},
		{"import without --file", []string{"enrollments", "import", "--preview"}, "--file"},
		{"preview without --file", []string{"enrollments", "preview-import"}, "--file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := loggedInByEnvironment(t)

			result := env.Run(tt.args...)

			if result.ExitCode != 1 || !strings.Contains(result.Stderr, tt.want) {
				t.Errorf("exit code = %d, stderr = %q; want 1 naming %s", result.ExitCode, result.Stderr, tt.want)
			}
			if routes := env.Transport.Routes(); len(routes) != 0 {
				t.Errorf("sent %v, want no requests", routes)
			}
		})
	}
}

func TestTheImportFixesValidateOnlyInsteadOfOfferingAFlag(t *testing.T) {
	for _, command := range []string{"import", "preview-import"} {
		env := testkit.NewEnv(t)

		result := env.Run("enrollments", command, "--help")

		if strings.Contains(result.Stdout, "validate-only") {
			t.Errorf("enrollments %s offers --options.validate-only:\n%s", command, result.Stdout)
		}
		if !strings.Contains(result.Stdout, "--options.course-id") {
			t.Errorf("enrollments %s lacks --options.course-id:\n%s", command, result.Stdout)
		}
	}
}

func TestAnEmptyPathArgumentIsAUsageError(t *testing.T) {
	env := loggedInByEnvironment(t)

	result := env.Run("learners", "retrieve", "")

	if result.ExitCode != 1 || !strings.Contains(result.Stderr, "<learnerId>") {
		t.Errorf("exit code = %d, stderr = %q; want 1 naming <learnerId>", result.ExitCode, result.Stderr)
	}
	if routes := env.Transport.Routes(); len(routes) != 0 {
		t.Errorf("sent %v, want no requests", routes)
	}
}
