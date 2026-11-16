package commands_test

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
)

const eventsListRoute = "GET /v1/organizations/" + testkit.OrgID + "/events"

// Three Learners across three pages; the second is a short page, which the
// API returns when a filter applies per page.
var learnerPages = []testkit.Reply{
	testkit.JSON(`{"items":[{"learnerId":"sub_1"},{"learnerId":"sub_2"}],"nextCursor":"c2"}`),
	testkit.JSON(`{"items":[],"nextCursor":"c3"}`),
	testkit.JSON(`{"items":[{"learnerId":"sub_3"}],"nextCursor":null}`),
}

func queries(t *testing.T, env *testkit.Env, route string) []url.Values {
	t.Helper()
	out := []url.Values{}
	for _, r := range env.Transport.Requests() {
		if r.Method+" "+r.Path != route {
			continue
		}
		parsed, err := url.Parse(r.URL)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, parsed.Query())
	}
	return out
}

func TestAllFollowsNextCursorUntilNullAndPrintsOneArray(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(learnersListRoute, learnerPages...)

	result := env.Run("learners", "list", "--all", "--limit", "2", "--status", "in_progress")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if want := `[{"learnerId":"sub_1"},{"learnerId":"sub_2"},{"learnerId":"sub_3"}]` + "\n"; result.Stdout != want {
		t.Errorf("stdout = %q\nwant     %q", result.Stdout, want)
	}
	got := queries(t, env, learnersListRoute)
	want := []string{"limit=2&status=in_progress", "cursor=c2&limit=2&status=in_progress", "cursor=c3&limit=2&status=in_progress"}
	if len(got) != len(want) {
		t.Fatalf("pages requested = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Encode() != want[i] {
			t.Errorf("page %d query = %s, want %s", i+1, got[i].Encode(), want[i])
		}
	}
}

func TestAllOnTheEventLogPagesAfterTheLastEventID(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(eventsListRoute,
		testkit.JSON(`{"items":[{"eventId":"evt_1","eventType":"learner.certified"},{"eventId":"evt_2","eventType":"learner.certified"}],"nextCursor":"opaque"}`),
		testkit.JSON(`{"items":[{"eventId":"evt_3","eventType":"learner.certified"}],"nextCursor":null}`))

	result := env.Run("events", "list", "--all", "--type", "learner.certified", "--after", "evt_0")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	got := queries(t, env, eventsListRoute)
	want := []string{"after=evt_0&type=learner.certified", "after=evt_2&type=learner.certified"}
	if len(got) != len(want) {
		t.Fatalf("pages requested = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Encode() != want[i] {
			t.Errorf("page %d query = %s, want %s", i+1, got[i].Encode(), want[i])
		}
	}
	if want := `[{"eventId":"evt_1","eventType":"learner.certified"},{"eventId":"evt_2","eventType":"learner.certified"},{"eventId":"evt_3","eventType":"learner.certified"}]` + "\n"; result.Stdout != want {
		t.Errorf("stdout = %q\nwant     %q", result.Stdout, want)
	}
}

func TestNDJSONPrintsOneCompactItemPerLine(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.StdoutIsTTY = true
	env.Vars["NO_COLOR"] = "1"
	env.Transport.Script(learnersListRoute, learnerPages...)

	result := env.Run("learners", "list", "--all", "--ndjson")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if want := "{\"learnerId\":\"sub_1\"}\n{\"learnerId\":\"sub_2\"}\n{\"learnerId\":\"sub_3\"}\n"; result.Stdout != want {
		t.Errorf("stdout = %q\nwant     %q", result.Stdout, want)
	}
}

func TestAllOnATerminalPrintsAnIndentedArray(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.StdoutIsTTY = true
	env.Vars["NO_COLOR"] = "1"
	env.Transport.Script(learnersListRoute, learnerPages...)

	result := env.Run("learners", "list", "--all")

	want := "[\n  {\n    \"learnerId\": \"sub_1\"\n  },\n  {\n    \"learnerId\": \"sub_2\"\n  },\n  {\n    \"learnerId\": \"sub_3\"\n  }\n]\n"
	if result.Stdout != want {
		t.Errorf("stdout = %q\nwant     %q", result.Stdout, want)
	}
}

func TestAllOfAnEmptyListPrintsAnEmptyArray(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(learnersListRoute, testkit.JSON(`{"items":[],"nextCursor":null}`))

	result := env.Run("learners", "list", "--all")

	if result.ExitCode != 0 || result.Stdout != "[]\n" {
		t.Errorf("exit code = %d, stdout = %q; want 0 and []", result.ExitCode, result.Stdout)
	}
}

func TestCtrlCStopsAllAndLeavesTheArrayUnfinished(t *testing.T) {
	env := loggedInByEnvironment(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env.Transport.Script(learnersListRoute, learnerPages[0], testkit.Reply{Before: cancel, Hang: true})

	result := env.RunContext(ctx, "learners", "list", "--all")

	if result.ExitCode != 130 {
		t.Errorf("exit code = %d, want 130; stderr = %q", result.ExitCode, result.Stderr)
	}
	if want := `[{"learnerId":"sub_1"},{"learnerId":"sub_2"}`; result.Stdout != want {
		t.Errorf("stdout = %q, want %q with no closing bracket", result.Stdout, want)
	}
	if n := countRoute(env, learnersListRoute); n != 2 {
		t.Errorf("pages requested = %d, want 2 and no retry", n)
	}
}

func TestNDJSONWithoutAllIsAUsageError(t *testing.T) {
	env := loggedInByEnvironment(t)

	result := env.Run("learners", "list", "--ndjson")

	if result.ExitCode != 1 || !strings.Contains(result.Stderr, "--all") {
		t.Errorf("exit code = %d, stderr = %q; want 1 naming --all", result.ExitCode, result.Stderr)
	}
	if routes := env.Transport.Routes(); len(routes) != 0 {
		t.Errorf("sent %v, want no requests", routes)
	}
}

func TestListHelpExplainsAllAndNDJSON(t *testing.T) {
	env := testkit.NewEnv(t)

	result := env.Run("learners", "list", "--help")

	for _, want := range []string{"--all", "--ndjson", "one JSON array", "one compact item per line"} {
		if !strings.Contains(result.Stdout, want) {
			t.Errorf("help does not contain %q:\n%s", want, result.Stdout)
		}
	}
}
