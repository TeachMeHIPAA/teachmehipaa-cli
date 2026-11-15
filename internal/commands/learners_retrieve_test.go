package commands_test

import (
	"encoding/base64"
	"testing"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
)

func TestLearnersRetrieveWithEnvironmentCredentials(t *testing.T) {
	env := testkit.NewEnv(t)
	env.Vars["TMH_CLIENT_ID"] = testkit.ClientID
	env.Vars["TMH_CLIENT_SECRET"] = testkit.ClientSecret
	env.Transport.
		Script(testkit.TokenRoute, testkit.TokenReply("jwt-1", 900)).
		Script(testkit.IntrospectionRoute, testkit.IntrospectionReply("test")).
		Script(testkit.LearnerRoute, testkit.JSON(testkit.LearnerBody))

	result := env.Run("learners", "retrieve", testkit.LearnerID)

	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if want := testkit.LearnerBody + "\n"; result.Stdout != want {
		t.Errorf("stdout = %q, want %q", result.Stdout, want)
	}
	requests := env.Transport.Requests()
	if len(requests) != 3 {
		t.Fatalf("routes = %v, want token, introspection, learner", env.Transport.Routes())
	}
	token, introspection, learner := requests[0], requests[1], requests[2]

	if token.URL != "https://api-teachmehipaa-dev.web.app/v1/auth/token" {
		t.Errorf("token URL = %s", token.URL)
	}
	basic := "Basic " + base64.StdEncoding.EncodeToString([]byte(testkit.ClientID+":"+testkit.ClientSecret))
	assertHeader(t, token, "Authorization", basic)
	assertHeader(t, token, "Content-Type", "application/x-www-form-urlencoded")
	if token.Body != "grant_type=client_credentials" {
		t.Errorf("token body = %q", token.Body)
	}

	if introspection.URL != "https://api-teachmehipaa-dev.web.app/v1/auth/introspection" {
		t.Errorf("introspection URL = %s", introspection.URL)
	}
	assertHeader(t, introspection, "Authorization", "Bearer jwt-1")

	if learner.URL != "https://api-teachmehipaa-dev.web.app/v1/organizations/org_acme/learners/"+testkit.LearnerID {
		t.Errorf("learner URL = %s", learner.URL)
	}
	assertHeader(t, learner, "Authorization", "Bearer jwt-1")
	assertHeader(t, learner, "Accept", "application/json")
}

func assertHeader(t *testing.T, r testkit.Request, name, want string) {
	t.Helper()
	if got := r.Header.Get(name); got != want {
		t.Errorf("%s %s header %s = %q, want %q", r.Method, r.Path, name, got, want)
	}
}
