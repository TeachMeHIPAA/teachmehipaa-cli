package commands_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/keyring"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
)

const (
	eventsRoute = "GET /v1/organizations/" + testkit.OrgID + "/events"
	eventsURL   = sandboxBase + "/organizations/" + testkit.OrgID + "/events"

	certifiedEvent = `{"eventId":"evt_01","eventType":"learner.certified","orgId":"org_acme","occurredAt":"2026-11-15T11:58:00.000Z","data":{"learnerId":"sub_1","email":"ana@example.com","externalId":null,"courseId":"hipaa-2026","status":"certified","occurredAt":"2026-11-15T11:58:00.000Z"}}`
	expiredEvent   = `{"eventId":"evt_02","eventType":"learner.expired","orgId":"org_acme","occurredAt":"2026-11-15T11:59:00.000Z","data":{"learnerId":"sub_2","email":"bo@example.com","externalId":"hr-2","courseId":"hipaa-2026","status":"expired","occurredAt":"2026-11-15T11:59:00.000Z"}}`
	invitedEvent   = `{"eventId":"evt_03","eventType":"learner.invited","orgId":"org_acme","occurredAt":"2026-11-15T12:00:00.000Z","data":{"learnerId":"sub_3","email":"cy@example.com","externalId":null,"courseId":"hipaa-2026","status":"invited","occurredAt":"2026-11-15T12:00:00.000Z"}}`

	// certifiedEventBody is certifiedEvent as a Delivery carries it:
	// canonical sorted-key JSON.
	certifiedEventBody = `{"data":{"courseId":"hipaa-2026","email":"ana@example.com","externalId":null,"learnerId":"sub_1","occurredAt":"2026-11-15T11:58:00.000Z","status":"certified"},"eventId":"evt_01","eventType":"learner.certified","occurredAt":"2026-11-15T11:58:00.000Z","orgId":"org_acme"}`
)

func eventsPage(nextCursor string, items ...string) testkit.Reply {
	cursor := "null"
	if nextCursor != "" {
		cursor = `"` + nextCursor + `"`
	}
	return testkit.JSON(`{"items":[` + strings.Join(items, ",") + `],"nextCursor":` + cursor + `}`)
}

// lastPage ends the tail the way Ctrl-C does, once the page is served.
func lastPage(cancel context.CancelFunc, items ...string) testkit.Reply {
	reply := eventsPage("", items...)
	reply.Before = cancel
	return reply
}

// target is the developer's local webhook handler.
type target struct {
	server   *httptest.Server
	mu       sync.Mutex
	received []received
	statuses []int
}

type received struct {
	path   string
	header http.Header
	body   string
}

// newTarget answers with statuses in turn, then 200.
func newTarget(t *testing.T, statuses ...int) *target {
	tg := &target{statuses: statuses}
	tg.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		tg.mu.Lock()
		tg.received = append(tg.received, received{path: r.URL.RequestURI(), header: r.Header.Clone(), body: string(body)})
		status := http.StatusOK
		if len(tg.statuses) > 0 {
			status, tg.statuses = tg.statuses[0], tg.statuses[1:]
		}
		tg.mu.Unlock()
		w.WriteHeader(status)
	}))
	t.Cleanup(tg.server.Close)
	return tg
}

func (tg *target) requests() []received {
	tg.mu.Lock()
	defer tg.mu.Unlock()
	return append([]received(nil), tg.received...)
}

// verifies checks a Delivery the way the SDKs' constructEvent does, from
// the platform's canonical string.
func (r received) verifies(secret string) bool {
	hash := sha256.Sum256([]byte(r.body))
	if hex.EncodeToString(hash[:]) != r.header.Get("X-Tmh-Body-Hash") {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("POST\n" + r.path + "\n" + r.header.Get("X-Tmh-Timestamp") + "\n" + r.header.Get("X-Tmh-Body-Hash")))
	return hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(r.header.Get("X-Tmh-Signature")))
}

func printSecret(t *testing.T, env *testkit.Env) string {
	t.Helper()
	result := env.Run("events", "tail", "--print-secret")
	if result.ExitCode != 0 {
		t.Fatalf("--print-secret exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	return strings.TrimSpace(result.Stdout)
}

func TestTailForwardsEachEventSignedWithTheLocalSigningSecret(t *testing.T) {
	env := loggedInByEnvironment(t)
	tg := newTarget(t)
	ctx, cancel := context.WithCancel(context.Background())
	env.Transport.Script(eventsRoute, eventsPage("", certifiedEvent), lastPage(cancel))

	result := env.RunContext(ctx, "events", "tail", "--from-start", "--forward-to", tg.server.URL+"/webhooks?source=tmh")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	secret := printSecret(t, env)
	if !strings.Contains(result.Stderr, "Ready! Your local signing secret is "+secret) {
		t.Errorf("stderr = %q, want the Ready line with the secret %q", result.Stderr, secret)
	}
	got := tg.requests()
	if len(got) != 1 {
		t.Fatalf("target got %d requests, want 1", len(got))
	}
	delivery := got[0]
	if delivery.body != certifiedEventBody {
		t.Errorf("body = %s\nwant   %s", delivery.body, certifiedEventBody)
	}
	if !delivery.verifies(secret) {
		t.Errorf("the forwarded Delivery doesn't verify with the printed secret; headers %v", delivery.header)
	}
	wantHeaders := map[string]string{
		"Content-Type":     "application/json",
		"X-Tmh-Key-Id":     "local",
		"X-Tmh-Timestamp":  "2026-11-15T12:00:00.000Z",
		"X-Tmh-Event-Type": "learner.certified",
		"X-Tmh-Event-Id":   "evt_01",
	}
	for name, want := range wantHeaders {
		if got := delivery.header.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if auth := delivery.header.Get("Authorization"); auth != "" {
		t.Errorf("the target got Authorization %q; the Access token must never leave for it", auth)
	}
	if want := "2026-11-15T11:58:00.000Z  learner.certified  evt_01  [200] POST " + tg.server.URL + "/webhooks?source=tmh (0ms)\n"; result.Stdout != want {
		t.Errorf("stdout = %q\nwant     %q", result.Stdout, want)
	}
	if want := eventsURL + "?limit=100"; env.Transport.Requests()[2].URL != want {
		t.Errorf("first events request = %s, want %s", env.Transport.Requests()[2].URL, want)
	}
}

func eventsRequestURLs(env *testkit.Env) []string {
	urls := []string{}
	for _, request := range env.Transport.Requests() {
		if request.Method+" "+request.Path == eventsRoute {
			urls = append(urls, request.URL)
		}
	}
	return urls
}

func TestTailResumesWhereTheLastTailStoppedWithoutDuplicates(t *testing.T) {
	env := loggedInByEnvironment(t)
	ctx, cancel := context.WithCancel(context.Background())
	env.Transport.Script(eventsRoute, eventsPage("", certifiedEvent, expiredEvent), lastPage(cancel))
	first := env.RunContext(ctx, "events", "tail", "--from-start")
	if first.ExitCode != 0 {
		t.Fatalf("first tail exit code = %d, stderr = %q", first.ExitCode, first.Stderr)
	}

	env.Transport.Script(testkit.TokenRoute, testkit.TokenReply("jwt-2", 900)).
		Script(testkit.IntrospectionRoute, testkit.IntrospectionReply("test"))
	ctx, cancel = context.WithCancel(context.Background())
	env.Transport.Script(eventsRoute, lastPage(cancel, invitedEvent))
	second := env.RunContext(ctx, "events", "tail")

	if second.ExitCode != 0 {
		t.Fatalf("second tail exit code = %d, stderr = %q", second.ExitCode, second.Stderr)
	}
	if want := "2026-11-15T11:58:00.000Z  learner.certified  evt_01\n2026-11-15T11:59:00.000Z  learner.expired  evt_02\n"; first.Stdout != want {
		t.Errorf("first tail stdout = %q\nwant %q", first.Stdout, want)
	}
	if want := "2026-11-15T12:00:00.000Z  learner.invited  evt_03\n"; second.Stdout != want {
		t.Errorf("second tail stdout = %q\nwant %q", second.Stdout, want)
	}
	urls := eventsRequestURLs(env)
	if want := eventsURL + "?after=evt_02&limit=100"; urls[len(urls)-1] != want {
		t.Errorf("second tail asked for %s, want %s", urls[len(urls)-1], want)
	}
}

func TestTailFirstRunStartsWithTheNextNewEvent(t *testing.T) {
	env := loggedInByEnvironment(t)
	ctx, cancel := context.WithCancel(context.Background())
	env.Transport.Script(eventsRoute,
		eventsPage("evt_01", certifiedEvent),
		eventsPage("", expiredEvent),
		lastPage(cancel, invitedEvent))

	result := env.RunContext(ctx, "events", "tail")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if want := "2026-11-15T12:00:00.000Z  learner.invited  evt_03\n"; result.Stdout != want {
		t.Errorf("stdout = %q, want only the Event that arrived after the start: %q", result.Stdout, want)
	}
	want := []string{eventsURL + "?limit=100", eventsURL + "?after=evt_01&limit=100", eventsURL + "?after=evt_02&limit=100"}
	if got := eventsRequestURLs(env); !slices.Equal(got, want) {
		t.Errorf("events requests = %v\nwant %v", got, want)
	}
}

func TestAfterOverridesWhereTheLastTailStopped(t *testing.T) {
	env := loggedInByEnvironment(t)
	ctx, cancel := context.WithCancel(context.Background())
	env.Transport.Script(eventsRoute, lastPage(cancel, certifiedEvent))
	env.RunContext(ctx, "events", "tail", "--from-start")

	env.Transport.Script(testkit.TokenRoute, testkit.TokenReply("jwt-2", 900)).
		Script(testkit.IntrospectionRoute, testkit.IntrospectionReply("test"))
	ctx, cancel = context.WithCancel(context.Background())
	env.Transport.Script(eventsRoute, lastPage(cancel))
	result := env.RunContext(ctx, "events", "tail", "--after", "evt_00")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	urls := eventsRequestURLs(env)
	if want := eventsURL + "?after=evt_00&limit=100"; urls[len(urls)-1] != want {
		t.Errorf("tail asked for %s, want %s", urls[len(urls)-1], want)
	}
}

func TestTailWaitsLongerWhileTheLogIsQuiet(t *testing.T) {
	env := loggedInByEnvironment(t)
	ctx, cancel := context.WithCancel(context.Background())
	env.Transport.Script(eventsRoute,
		eventsPage(""), eventsPage(""), eventsPage(""), eventsPage(""),
		eventsPage("", certifiedEvent), lastPage(cancel))

	result := env.RunContext(ctx, "events", "tail", "--from-start")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	want := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second, 10 * time.Second, 2 * time.Second, 2 * time.Second}
	if got := env.Clock.Sleeps(); !slices.Equal(got, want) {
		t.Errorf("waits = %v, want %v", got, want)
	}
}

func TestOneTypeIsFilteredByTheAPI(t *testing.T) {
	env := loggedInByEnvironment(t)
	ctx, cancel := context.WithCancel(context.Background())
	env.Transport.Script(eventsRoute, lastPage(cancel, certifiedEvent))

	result := env.RunContext(ctx, "events", "tail", "--from-start", "--types", "learner.certified")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if want := eventsURL + "?limit=100&type=learner.certified"; eventsRequestURLs(env)[0] != want {
		t.Errorf("events request = %s, want %s", eventsRequestURLs(env)[0], want)
	}
}

func TestSeveralTypesAreFilteredLocallyAndSkippedEventsStillCount(t *testing.T) {
	env := loggedInByEnvironment(t)
	tg := newTarget(t)
	ctx, cancel := context.WithCancel(context.Background())
	env.Transport.Script(eventsRoute, eventsPage("", certifiedEvent, expiredEvent, invitedEvent), lastPage(cancel))

	result := env.RunContext(ctx, "events", "tail", "--from-start",
		"--types", "learner.certified,learner.invited", "--forward-to", tg.server.URL)

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	urls := eventsRequestURLs(env)
	if want := eventsURL + "?limit=100"; urls[0] != want {
		t.Errorf("events request = %s, want %s (the API filters one type only)", urls[0], want)
	}
	if want := eventsURL + "?after=evt_03&limit=100"; urls[1] != want {
		t.Errorf("next events request = %s, want %s", urls[1], want)
	}
	forwarded := []string{}
	for _, r := range tg.requests() {
		forwarded = append(forwarded, r.header.Get("X-Tmh-Event-Id"))
	}
	if want := []string{"evt_01", "evt_03"}; !slices.Equal(forwarded, want) {
		t.Errorf("forwarded %v, want %v", forwarded, want)
	}
	if strings.Contains(result.Stdout, "evt_02") {
		t.Errorf("stdout = %q shows a filtered-out Event", result.Stdout)
	}
}

func TestAForwardTargetFailureDoesNotStopTheTail(t *testing.T) {
	env := loggedInByEnvironment(t)
	tg := newTarget(t, http.StatusInternalServerError)
	ctx, cancel := context.WithCancel(context.Background())
	env.Transport.Script(eventsRoute, eventsPage("", certifiedEvent, expiredEvent), lastPage(cancel))

	result := env.RunContext(ctx, "events", "tail", "--from-start", "--forward-to", tg.server.URL+"/hooks")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	want := "2026-11-15T11:58:00.000Z  learner.certified  evt_01  [500] POST " + tg.server.URL + "/hooks (0ms)\n" +
		"2026-11-15T11:59:00.000Z  learner.expired  evt_02  [200] POST " + tg.server.URL + "/hooks (0ms)\n"
	if result.Stdout != want {
		t.Errorf("stdout = %q\nwant     %q", result.Stdout, want)
	}
}

func TestAnUnreachableForwardTargetDoesNotStopTheTail(t *testing.T) {
	env := loggedInByEnvironment(t)
	tg := newTarget(t)
	tg.server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	env.Transport.Script(eventsRoute, eventsPage("", certifiedEvent, expiredEvent), lastPage(cancel))

	result := env.RunContext(ctx, "events", "tail", "--from-start", "--forward-to", tg.server.URL)

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	lines := strings.Split(strings.TrimSpace(result.Stdout), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "evt_01  [error] POST "+tg.server.URL) || !strings.Contains(lines[1], "evt_02  [error]") {
		t.Errorf("stdout = %q, want an [error] line per Event", result.Stdout)
	}
}

func TestCtrlCDuringAForwardLeavesThatEventForTheNextTail(t *testing.T) {
	env := loggedInByEnvironment(t)
	ctx, cancel := context.WithCancel(context.Background())
	release := make(chan struct{})
	hanging := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		cancel()
		<-release
	}))
	t.Cleanup(hanging.Close)
	t.Cleanup(func() { close(release) })
	env.Transport.Script(eventsRoute, eventsPage("", certifiedEvent))
	first := env.RunContext(ctx, "events", "tail", "--from-start", "--forward-to", hanging.URL)

	if first.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", first.ExitCode, first.Stderr)
	}
	env.Transport.Script(testkit.TokenRoute, testkit.TokenReply("jwt-2", 900)).
		Script(testkit.IntrospectionRoute, testkit.IntrospectionReply("test"))
	ctx, cancel = context.WithCancel(context.Background())
	env.Transport.Script(eventsRoute, lastPage(cancel))
	env.RunContext(ctx, "events", "tail")
	urls := eventsRequestURLs(env)
	if want := eventsURL + "?after=evt_01&limit=100"; slices.Contains(urls, want) {
		t.Errorf("the interrupted Event counted as seen: requests %v", urls)
	}
}

func TestAnAPIErrorEndsTheTailButKeepsItsPlace(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(eventsRoute, eventsPage("", certifiedEvent),
		testkit.Reply{Status: 403, Header: map[string]string{"Content-Type": "application/problem+json"},
			Body: `{"type":"about:blank","title":"Forbidden","status":403,"detail":"Missing scope webhooks:manage.","code":"insufficient_scope"}`})

	result := env.Run("events", "tail", "--from-start")

	if result.ExitCode != 2 {
		t.Errorf("exit code = %d, want 2; stderr = %q", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(result.Stderr, "Error: Forbidden: Missing scope webhooks:manage.") {
		t.Errorf("stderr = %q, want the rendered problem", result.Stderr)
	}
	env.Transport.Script(testkit.TokenRoute, testkit.TokenReply("jwt-2", 900)).
		Script(testkit.IntrospectionRoute, testkit.IntrospectionReply("test"))
	ctx, cancel := context.WithCancel(context.Background())
	env.Transport.Script(eventsRoute, lastPage(cancel))
	env.RunContext(ctx, "events", "tail")
	urls := eventsRequestURLs(env)
	if want := eventsURL + "?after=evt_01&limit=100"; urls[len(urls)-1] != want {
		t.Errorf("next tail asked for %s, want %s", urls[len(urls)-1], want)
	}
}

func TestTheLocalSigningSecretIsStablePerProfile(t *testing.T) {
	env := testkit.NewEnv(t)

	first := printSecret(t, env)
	second := printSecret(t, env)
	live := env.Run("events", "tail", "--print-secret", "--live")

	if first != second {
		t.Errorf("secret changed between runs: %q then %q", first, second)
	}
	if decoded, err := base64.StdEncoding.DecodeString(first); err != nil || len(decoded) != 32 {
		t.Errorf("secret %q isn't 32 bytes of base64 like a Signing secret", first)
	}
	if strings.TrimSpace(live.Stdout) == first {
		t.Error("the live profile shares the sandbox profile's secret")
	}
}

func TestTheLocalSigningSecretFallsBackToTheConfigFile(t *testing.T) {
	env := testkit.NewEnv(t)
	env.Keyring = keyring.NewUnavailable()

	first := printSecret(t, env)
	second := printSecret(t, env)

	if first == "" || first != second {
		t.Errorf("secret = %q then %q, want one stable secret", first, second)
	}
	info, err := os.Stat(filepath.Join(env.ConfigDir, "config.toml"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("config.toml = %v, %v; want it written 0600", info, err)
	}
}

func TestResendForwardsOneEventFromTheLog(t *testing.T) {
	env := loggedInByEnvironment(t)
	tg := newTarget(t)
	env.Transport.Script(eventsRoute, eventsPage("evt_02", invitedEvent, expiredEvent), eventsPage("", certifiedEvent))

	result := env.Run("events", "resend", "evt_01", "--forward-to", tg.server.URL+"/webhooks")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	got := tg.requests()
	if len(got) != 1 || got[0].body != certifiedEventBody || !got[0].verifies(printSecret(t, env)) {
		t.Fatalf("target got %+v, want evt_01 signed with the local secret", got)
	}
	if want := []string{eventsURL + "?limit=100", eventsURL + "?after=evt_02&limit=100"}; !slices.Equal(eventsRequestURLs(env), want) {
		t.Errorf("events requests = %v, want %v", eventsRequestURLs(env), want)
	}
	if want := "2026-11-15T11:58:00.000Z  learner.certified  evt_01  [200] POST " + tg.server.URL + "/webhooks (0ms)\n"; result.Stdout != want {
		t.Errorf("stdout = %q\nwant     %q", result.Stdout, want)
	}
}

func TestResendOfAnEventNotInTheLogExitsNotFound(t *testing.T) {
	env := loggedInByEnvironment(t)
	tg := newTarget(t)
	env.Transport.Script(eventsRoute, eventsPage("", certifiedEvent))

	result := env.Run("events", "resend", "evt_99", "--forward-to", tg.server.URL)

	if result.ExitCode != 3 {
		t.Errorf("exit code = %d, want 3; stderr = %q", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(result.Stderr, "evt_99") || len(tg.requests()) != 0 {
		t.Errorf("stderr = %q, target got %d requests", result.Stderr, len(tg.requests()))
	}
}

func TestResendFailsWhenTheTargetRejectsTheEvent(t *testing.T) {
	env := loggedInByEnvironment(t)
	tg := newTarget(t, http.StatusBadRequest)
	env.Transport.Script(eventsRoute, eventsPage("", certifiedEvent))

	result := env.Run("events", "resend", "evt_01", "--forward-to", tg.server.URL)

	if result.ExitCode != 1 || !strings.Contains(result.Stdout, "[400]") {
		t.Errorf("exit code = %d, stdout = %q; want 1 and the [400]", result.ExitCode, result.Stdout)
	}
}

func TestForwardToMustBeAnHTTPURL(t *testing.T) {
	env := testkit.NewEnv(t)

	result := env.Run("events", "tail", "--forward-to", "localhost:3000/webhooks")

	if result.ExitCode != 1 || !strings.Contains(result.Stderr, "--forward-to") {
		t.Errorf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
}

func TestTriggerTestSendsATestDeliveryToTheWebhookSubscription(t *testing.T) {
	env := loggedInByEnvironment(t)
	route := "POST /v1/organizations/" + testkit.OrgID + "/webhook-subscriptions/" + testkit.SubscriptionID + ":test"
	operation := `{"name":"operations/op_1","done":true,"response":{"_tag":"sent","status":204}}`
	env.Transport.Script(route, testkit.Reply{Status: 202, Body: operation})

	result := env.Run("webhooks", "trigger-test", testkit.SubscriptionID)

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if result.Stdout != operation+"\n" {
		t.Errorf("stdout = %q, want the Operation", result.Stdout)
	}
	if !uuidV4.MatchString(env.Transport.Requests()[2].Header.Get("Idempotency-Key")) {
		t.Error("trigger-test sent no generated Idempotency-Key")
	}
}

func TestCtrlCWhileFindingTheStartExitsCleanly(t *testing.T) {
	env := loggedInByEnvironment(t)
	ctx, cancel := context.WithCancel(context.Background())
	env.Transport.Script(eventsRoute, eventsPage("evt_01", certifiedEvent), testkit.Reply{Hang: true, Before: cancel})

	result := env.RunContext(ctx, "events", "tail")

	if result.ExitCode != 0 || result.Stdout != "" {
		t.Errorf("exit code = %d, stdout = %q, stderr = %q; want a quiet exit 0", result.ExitCode, result.Stdout, result.Stderr)
	}
}

func TestEventsListTakesCursorAsAnotherNameForAfter(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(eventsRoute, eventsPage("", expiredEvent))

	result := env.Run("events", "list", "--cursor", "evt_01", "--limit", "1")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	requests := env.Transport.Requests()
	if got, want := requests[len(requests)-1].URL, eventsURL+"?after=evt_01&limit=1"; got != want {
		t.Errorf("URL = %s, want %s", got, want)
	}
}
