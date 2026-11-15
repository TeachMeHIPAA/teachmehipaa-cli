package commands_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
)

const (
	exportEnrollmentsRoute = "POST /v1/organizations/" + testkit.OrgID + "/enrollments:export"
	operationName          = "organizations/" + testkit.OrgID + "/operations/op_01JD84"
	operationRoute         = "GET /v1/" + operationName
	pendingOperation       = `{"name":"` + operationName + `","done":false,"metadata":{"kind":"enrollments.export"}}`
	// exportCSV is what exportedOperation's dataBase64 decodes to.
	exportCSV         = "email,name\r\nana@example.com,Ana Example\r\nben@example.com,\"Ben, Jr.\"\r\n"
	exportedOperation = `{"name":"` + operationName + `","done":true,"metadata":{"kind":"enrollments.export"},` +
		`"response":{"format":"csv","contentType":"text/csv","filename":"enrollments-2026-11-15.csv","rowCount":2,` +
		`"dataBase64":"ZW1haWwsbmFtZQ0KYW5hQGV4YW1wbGUuY29tLEFuYSBFeGFtcGxlDQpiZW5AZXhhbXBsZS5jb20sIkJlbiwgSnIuIg0K"}}`
	failedOperation = `{"name":"` + operationName + `","done":true,"metadata":{"kind":"enrollments.export"},` +
		`"error":{"code":"export_too_large","title":"Export too large","status":422,"detail":"Narrow the filter to under 10000 rows."}}`
)

func accepted(body string) testkit.Reply { return testkit.Reply{Status: 202, Body: body} }

func TestWaitOnAnOperationThatIsAlreadyDonePrintsItWithoutPolling(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(exportEnrollmentsRoute, accepted(exportedOperation))

	result := env.Run("enrollments", "export", "--format", "csv", "--wait")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if result.Stdout != exportedOperation+"\n" {
		t.Errorf("stdout = %q, want the Operation", result.Stdout)
	}
	if n := countRoute(env, operationRoute); n != 0 {
		t.Errorf("polls = %d, want none", n)
	}
}

func TestWaitPollsWithBackoffUntilTheOperationIsDone(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.
		Script(exportEnrollmentsRoute, accepted(pendingOperation)).
		Script(operationRoute,
			testkit.JSON(pendingOperation), testkit.JSON(pendingOperation), testkit.JSON(pendingOperation),
			testkit.JSON(pendingOperation), testkit.JSON(pendingOperation), testkit.JSON(exportedOperation))

	result := env.Run("enrollments", "export", "--format", "csv", "--wait")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if result.Stdout != exportedOperation+"\n" {
		t.Errorf("stdout = %q, want the finished Operation", result.Stdout)
	}
	// The SDKs' wait(): the first wait is 1 s, the ceiling doubles to 10 s,
	// jitter picks 50–100 % of it, and nothing waits under 1 s.
	bounds := [][2]time.Duration{
		{time.Second, time.Second},
		{time.Second, 2 * time.Second},
		{2 * time.Second, 4 * time.Second},
		{4 * time.Second, 8 * time.Second},
		{5 * time.Second, 10 * time.Second},
		{5 * time.Second, 10 * time.Second},
	}
	sleeps := env.Clock.Sleeps()
	if len(sleeps) != len(bounds) {
		t.Fatalf("sleeps = %v, want %d", sleeps, len(bounds))
	}
	for i, b := range bounds {
		if sleeps[i] < b[0] || sleeps[i] > b[1] {
			t.Errorf("wait %d = %v, want %v–%v", i+1, sleeps[i], b[0], b[1])
		}
	}
	if n := countRoute(env, operationRoute); n != 6 {
		t.Errorf("polls = %d, want 6", n)
	}
}

func TestWaitOnAFailedOperationPrintsItAndRendersTheProblem(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.
		Script(exportEnrollmentsRoute, accepted(pendingOperation)).
		Script(operationRoute, testkit.JSON(failedOperation))

	result := env.Run("enrollments", "export", "--format", "csv", "--wait")

	if result.ExitCode != 5 {
		t.Errorf("exit code = %d, want 5 for its 422", result.ExitCode)
	}
	if result.Stdout != failedOperation+"\n" {
		t.Errorf("stdout = %q, want the failed Operation", result.Stdout)
	}
	for _, want := range []string{operationName, "Export too large", "Narrow the filter to under 10000 rows."} {
		if !strings.Contains(result.Stderr, want) {
			t.Errorf("stderr = %q, want it to contain %q", result.Stderr, want)
		}
	}
}

func TestAFailedOperationExitsLikeAnHTTPErrorWithItsProblemsStatus(t *testing.T) {
	tests := []struct {
		name     string
		problem  string
		wantExit int
	}{
		{"422 invalid roster", `{"code":"invalid_csv","title":"Invalid roster","status":422}`, 5},
		{"409 conflict", `{"code":"seat_unavailable","title":"No Seats left","status":409}`, 4},
		{"403 forbidden", `{"code":"insufficient_scope","title":"Forbidden","status":403}`, 2},
		{"500 internal", `{"code":"internal","title":"Internal error","status":500}`, 7},
		{"no status", `{"code":"internal","title":"Internal error"}`, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := loggedInByEnvironment(t)
			failed := `{"name":"` + operationName + `","done":true,"error":` + tt.problem + `}`
			env.Transport.Script(importRoute, accepted(failed))

			result := env.Run("enrollments", "import", "--file", writeRoster(t), "--wait")

			if result.ExitCode != tt.wantExit {
				t.Errorf("exit code = %d, want %d; stderr = %q", result.ExitCode, tt.wantExit, result.Stderr)
			}
		})
	}
}

func TestWaitGivesUpAtTheTimeout(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(exportEnrollmentsRoute, accepted(pendingOperation))
	for range 10 {
		env.Transport.Script(operationRoute, testkit.JSON(pendingOperation))
	}

	result := env.Run("enrollments", "export", "--format", "csv", "--wait", "--wait-timeout", "3s")

	if result.ExitCode != 7 {
		t.Errorf("exit code = %d, want 7; stderr = %q", result.ExitCode, result.Stderr)
	}
	for _, want := range []string{operationName, "3s", "tmh operations retrieve op_01JD84 --wait"} {
		if !strings.Contains(result.Stderr, want) {
			t.Errorf("stderr = %q, want it to contain %q", result.Stderr, want)
		}
	}
	total := time.Duration(0)
	for _, sleep := range env.Clock.Sleeps() {
		total += sleep
	}
	if total != 3*time.Second {
		t.Errorf("waited %v in all (%v), want exactly the 3s timeout", total, env.Clock.Sleeps())
	}
}

func TestOperationsRetrieveCanWait(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(operationRoute, testkit.JSON(pendingOperation), testkit.JSON(exportedOperation))

	result := env.Run("operations", "retrieve", "op_01JD84", "--wait")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if result.Stdout != exportedOperation+"\n" {
		t.Errorf("stdout = %q, want the finished Operation", result.Stdout)
	}
}

func TestExportOutputWritesTheDecodedCSV(t *testing.T) {
	out := filepath.Join(t.TempDir(), "roster.csv")
	env := loggedInByEnvironment(t)
	env.Transport.
		Script(exportEnrollmentsRoute, accepted(pendingOperation)).
		Script(operationRoute, testkit.JSON(exportedOperation))

	result := env.Run("enrollments", "export", "--format", "csv", "--wait", "-o", out)

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != exportCSV {
		t.Errorf("file = %q, want %q", written, exportCSV)
	}
	if result.Stdout != "" {
		t.Errorf("stdout = %q, want nothing", result.Stdout)
	}
	for _, want := range []string{out, "enrollments-2026-11-15.csv", "2 rows"} {
		if !strings.Contains(result.Stderr, want) {
			t.Errorf("stderr = %q, want it to contain %q", result.Stderr, want)
		}
	}
}

func TestExportOutputToStdoutPrintsTheCSV(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.StdoutIsTTY = true
	env.Transport.Script(exportEnrollmentsRoute, accepted(exportedOperation))

	result := env.Run("enrollments", "export", "--format", "csv", "-o", "-")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if result.Stdout != exportCSV {
		t.Errorf("stdout = %q, want the CSV", result.Stdout)
	}
}

func TestCtrlCStopsWaiting(t *testing.T) {
	env := loggedInByEnvironment(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env.Transport.
		Script(exportEnrollmentsRoute, accepted(pendingOperation)).
		Script(operationRoute, testkit.Reply{Before: cancel, Hang: true})

	result := env.RunContext(ctx, "enrollments", "export", "--format", "csv", "--wait")

	if result.ExitCode != 130 || result.Stderr != "Interrupted\n" {
		t.Errorf("exit code = %d, stderr = %q; want 130 and Interrupted", result.ExitCode, result.Stderr)
	}
	if n := countRoute(env, operationRoute); n != 1 {
		t.Errorf("polls = %d, want 1 and no retry", n)
	}
}
