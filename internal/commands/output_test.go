package commands_test

import (
	"testing"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
)

func TestLearnerOutputDependsOnTheTerminal(t *testing.T) {
	const course = `{"courseId":"hipaa-2026","done":true,"score":97.5,"certificate":null,"note":"say \"hi\": ok"}`
	const pretty = "{\n" +
		`  "courseId": "hipaa-2026",` + "\n" +
		`  "done": true,` + "\n" +
		`  "score": 97.5,` + "\n" +
		`  "certificate": null,` + "\n" +
		`  "note": "say \"hi\": ok"` + "\n" +
		"}\n"
	const colored = "{\n" +
		"  \x1b[34m\"courseId\"\x1b[0m: \x1b[32m\"hipaa-2026\"\x1b[0m,\n" +
		"  \x1b[34m\"done\"\x1b[0m: \x1b[33mtrue\x1b[0m,\n" +
		"  \x1b[34m\"score\"\x1b[0m: \x1b[36m97.5\x1b[0m,\n" +
		"  \x1b[34m\"certificate\"\x1b[0m: \x1b[90mnull\x1b[0m,\n" +
		"  \x1b[34m\"note\"\x1b[0m: \x1b[32m\"say \\\"hi\\\": ok\"\x1b[0m\n" +
		"}\n"

	tests := []struct {
		name    string
		tty     bool
		noColor bool
		want    string
	}{
		{name: "piped stdout gets compact JSON", tty: false, want: course + "\n"},
		{name: "a terminal gets pretty, colored JSON", tty: true, want: colored},
		{name: "NO_COLOR keeps it pretty but plain", tty: true, noColor: true, want: pretty},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := loggedInByEnvironment(t)
			env.StdoutIsTTY = tt.tty
			if tt.noColor {
				env.Vars["NO_COLOR"] = "1"
			}
			env.Transport.Script(testkit.LearnerRoute, testkit.JSON(course))

			result := env.Run("learners", "retrieve", testkit.LearnerID)

			if result.ExitCode != 0 {
				t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
			}
			if result.Stdout != tt.want {
				t.Errorf("stdout =\n%q\nwant\n%q", result.Stdout, tt.want)
			}
		})
	}
}

func TestLearnersRetrieveSendsTheCourse(t *testing.T) {
	env := loggedInByEnvironment(t)
	env.Transport.Script(testkit.LearnerRoute, testkit.JSON(testkit.LearnerBody))

	result := env.Run("learners", "retrieve", testkit.LearnerID, "--course-id", "hipaa 2026")

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	learner := env.Transport.Requests()[2]
	want := "https://api-teachmehipaa-dev.web.app/v1/organizations/org_acme/learners/" + testkit.LearnerID + "?courseId=hipaa+2026"
	if learner.URL != want {
		t.Errorf("learner URL = %s, want %s", learner.URL, want)
	}
}

// loggedInByEnvironment has CI-style credentials and scripts the token and
// introspection calls every command starts with.
func loggedInByEnvironment(t *testing.T) *testkit.Env {
	t.Helper()
	env := testkit.NewEnv(t)
	env.Vars["TMH_CLIENT_ID"] = testkit.ClientID
	env.Vars["TMH_CLIENT_SECRET"] = testkit.ClientSecret
	env.Transport.
		Script(testkit.TokenRoute, testkit.TokenReply("jwt-1", 900)).
		Script(testkit.IntrospectionRoute, testkit.IntrospectionReply("test"))
	return env
}
