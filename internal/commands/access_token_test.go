package commands_test

import (
	"strings"
	"testing"
	"time"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
)

func TestTheAccessTokenIsReusedUntil60SecondsBeforeItExpires(t *testing.T) {
	tests := []struct {
		name       string
		elapsed    time.Duration
		wantRoutes []string
		wantBearer string
	}{
		{
			name:       "839 s into a 900 s token",
			elapsed:    839 * time.Second,
			wantRoutes: []string{testkit.TokenRoute, testkit.IntrospectionRoute, testkit.LearnerRoute},
			wantBearer: "Bearer jwt-1",
		},
		{
			name:       "840 s into a 900 s token",
			elapsed:    840 * time.Second,
			wantRoutes: []string{testkit.TokenRoute, testkit.IntrospectionRoute, testkit.TokenRoute, testkit.LearnerRoute},
			wantBearer: "Bearer jwt-2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := testkit.NewEnv(t)
			env.Vars["TMH_CLIENT_ID"] = testkit.ClientID
			env.Vars["TMH_CLIENT_SECRET"] = testkit.ClientSecret
			introspection := testkit.IntrospectionReply("test")
			introspection.Before = func() { env.Clock.Advance(tt.elapsed) }
			env.Transport.
				Script(testkit.TokenRoute, testkit.TokenReply("jwt-1", 900), testkit.TokenReply("jwt-2", 900)).
				Script(testkit.IntrospectionRoute, introspection).
				Script(testkit.LearnerRoute, testkit.JSON(testkit.LearnerBody))

			result := env.Run("learners", "retrieve", testkit.LearnerID)

			if result.ExitCode != 0 {
				t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
			}
			if got := env.Transport.Routes(); strings.Join(got, ",") != strings.Join(tt.wantRoutes, ",") {
				t.Errorf("routes = %v, want %v", got, tt.wantRoutes)
			}
			requests := env.Transport.Requests()
			assertHeader(t, requests[len(requests)-1], "Authorization", tt.wantBearer)
		})
	}
}
