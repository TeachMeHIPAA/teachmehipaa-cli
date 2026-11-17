package commands_test

import (
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
)

// sessions set up the fake API behind each console session in the docs,
// keyed by the name in its <!-- doctest: <name> --> annotation. Bodies
// follow the Developer API's schemas in openapi/sdk-ir.json.
var sessions = map[string]func(s *docSession){
	"readme-quickstart": func(s *docSession) {
		// The README logs in with `tmh login`, so the session's commands
		// read stored credentials rather than TMH_CLIENT_ID.
		s.storedLogin()
		s.scriptLogins("test", 2)
		s.env.Transport.Script("GET "+orgPath+"/compliance", testkit.JSON(complianceSummary))
	},
}

const orgPath = "/v1/organizations/" + testkit.OrgID

const complianceSummary = `{"asOf":"2026-11-15T12:00:00.000Z","courseId":"hipaa-2026",` +
	`"totals":{"learners":42,"compliant":30,"expiringSoon":3,"expired":1,"inProgress":5,` +
	`"notStarted":2,"inviteBounced":1},"compliantRate":0.714}`

// storedLogin runs `tmh login` for the sandbox profile, so the session's
// commands read the client secret from the keyring.
func (s *docSession) storedLogin() {
	s.scriptLogins("test", 1)
	s.env.Stdin = testkit.ClientSecret
	if login := s.env.Run("login", "--client-id", testkit.ClientID); login.ExitCode != 0 {
		s.t.Fatalf("tmh login: %q", login.Stderr)
	}
	s.env.Stdin = ""
}

// scriptLogins answers the token and introspection requests that each of n
// commands makes first.
func (s *docSession) scriptLogins(mode string, n int) {
	for range n {
		s.env.Transport.
			Script(testkit.TokenRoute, testkit.TokenReply("jwt-1", 900)).
			Script(testkit.IntrospectionRoute, testkit.IntrospectionReply(mode))
	}
}
