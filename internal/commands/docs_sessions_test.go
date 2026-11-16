package commands_test

import (
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/version"
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
	"version": func(s *docSession) {
		s.env.Build = version.Info{Version: "1.0.0", Commit: "39276f1", Date: "2026-11-15T19:27:47Z"}
	},
	"invite": func(s *docSession) {
		s.loggedIn("test")
		s.env.Transport.Script("POST "+orgPath+"/learners", testkit.Reply{Status: 201, Body: `{"learnerId":"sub_01JD8A7Y3K2M4N5P6Q7R8S9T0V",` +
			`"email":"ben@example.com","name":"Ben Example","externalId":null,"status":"not_started",` +
			`"course":{"courseId":"hipaa-2026","title":"HIPAA Training 2026"},"invitedAt":"2026-11-15T12:00:00.000Z",` +
			`"startedAt":null,"completedAt":null,"dueAt":"2026-12-15T00:00:00.000Z","certificate":null,"delivery":"email"}`})
	},
	"recipe-not-found": func(s *docSession) {
		s.loggedIn("test")
		s.env.Transport.Script("GET "+orgPath+"/learners/sub_01JD8ZZZZZZZZZZZZZZZZZZZZZ",
			docProblem(404, "req_01JD8Q", `{"code":"not_found","title":"Not found","status":404,"detail":"No Learner with that id."}`))
	},
	"login-piped": func(s *docSession) {
		s.env.Vars["TMH_SECRET"] = testkit.ClientSecret
		s.scriptLogins("test", 1)
	},
	"logout":        func(s *docSession) { s.storedLogin() },
	"not-logged-in": func(*docSession) {},
	"env-credentials": func(s *docSession) {
		s.scriptLogins("live", 1)
	},
	"missing-scope": func(s *docSession) {
		s.loggedIn("test")
		s.env.Transport.Script("GET "+orgPath+"/certificates", docProblem(403, "req_01JD8S",
			`{"code":"forbidden","title":"Forbidden","status":403,"detail":"The access token lacks the certificates:read scope."}`))
	},
	"public-lookup": func(s *docSession) {
		// Nothing else is scripted: a token request would fail the test.
		s.env.Transport.Script("GET /v1/certificate-verifications/7Hk2pQ", testkit.JSON(`{"status":"valid",`+
			`"learnerName":"Ana Example","courseTitle":"HIPAA Training 2026","issuedAt":"2026-11-10T15:00:00.000Z",`+
			`"expiresAt":"2027-11-10T15:00:00.000Z","issuer":"TeachMeHIPAA"}`))
	},
	"update-if-match": func(s *docSession) {
		s.loggedIn("test")
		s.env.Transport.Script("PATCH /v1/organizations/"+testkit.OrgID+"/learners/"+testkit.LearnerID,
			testkit.JSON(strings.Replace(testkit.LearnerBody, `"Ana Example"`, `"Ana B. Example"`, 1)))
	},
	"update-precondition-failed": func(s *docSession) {
		s.loggedIn("test")
		s.env.Transport.Script("PATCH /v1/organizations/"+testkit.OrgID+"/learners/"+testkit.LearnerID,
			docProblem(412, "req_01JD8T", `{"code":"precondition_failed","title":"Precondition failed","status":412,`+
				`"detail":"The learner changed since you read it. Retrieve it again for its current ETag."}`))
	},
	"raw-get": func(s *docSession) {
		s.loggedIn("test")
		s.env.Transport.Script(listLearnersRoute, testkit.JSON(`{"items":[{"learnerId":"sub_01JD80AB2C3D4E5F6G7H8J9K0N",`+
			`"email":"cam@example.com","name":"Cam Example","externalId":"hr-125","status":"compliant",`+
			`"course":{"courseId":"hipaa-2026","title":"HIPAA Training 2026"},"invitedAt":"2026-10-01T09:00:00.000Z",`+
			`"startedAt":"2026-10-02T10:30:00.000Z","completedAt":"2026-10-03T16:45:00.000Z",`+
			`"dueAt":"2026-11-01T00:00:00.000Z","certificate":{"certificateId":"cert_01JD85",`+
			`"issuedAt":"2026-10-03T16:45:00.000Z","expiresAt":"2027-10-03T16:45:00.000Z",`+
			`"verifyUrl":"https://tme.so/7Hk2pQ",`+
			`"pdfUrl":"https://api.tme.so/v1/organizations/org_acme/certificates/cert_01JD85/content"}}],"nextCursor":null}`))
	},
	"compact-output": func(s *docSession) {
		s.loggedIn("test")
		s.env.Transport.Script(testkit.LearnerRoute, testkit.JSON(testkit.LearnerBody))
	},
	"list-all":    expiredLearnerPages,
	"list-ndjson": expiredLearnerPages,
	"verbose": func(s *docSession) {
		s.env.Vars["TMH_CLIENT_ID"] = testkit.ClientID
		s.env.Vars["TMH_CLIENT_SECRET"] = testkit.ClientSecret
		token := testkit.TokenReply("jwt-1", 900)
		token.Header["X-Request-Id"], token.Before = "req_01JD8P", s.takes(84)
		introspection := testkit.IntrospectionReply("test")
		introspection.Header = map[string]string{"X-Request-Id": "req_01JD8Q"}
		introspection.Before = s.takes(35)
		learner := testkit.JSON(testkit.LearnerBody)
		learner.Header = map[string]string{"X-Request-Id": "req_01JD8R"}
		learner.Before = s.takes(41)
		s.env.Transport.
			Script(testkit.TokenRoute, token).
			Script(testkit.IntrospectionRoute, introspection).
			Script(testkit.LearnerRoute, learner)
	},
	"events-list": func(s *docSession) {
		s.loggedIn("test")
		s.env.Transport.Script(eventsRoute, eventsPage("", event("evt_01JD8M", "learner.certified", "11:58:00", "compliant")))
	},
	"tail": func(s *docSession) {
		s.loggedIn("test")
		s.localSigningSecret()
		s.env.Transport.Script(eventsRoute,
			eventsPage(""),
			eventsPage("", event("evt_01JD8N", "learner.invited", "12:00:00", "not_started"),
				event("evt_01JD8P", "learner.started", "12:00:04", "in_progress")),
			lastPage(s.cancel))
	},
	"tail-forward": func(s *docSession) {
		s.loggedIn("test")
		s.localSigningSecret()
		s.env.Transport.Script(eventsRoute,
			eventsPage(""),
			eventsPage("", event("evt_01JD8M", "learner.certified", "12:00:00", "compliant"),
				event("evt_01JD8N", "learner.expired", "12:00:04", "expired")),
			lastPage(s.cancel))
		s.forwardsTo(testkit.Reply{Status: 200, Before: s.takes(12)}, testkit.Reply{Status: 500, Before: s.takes(8)})
	},
	"resend": func(s *docSession) {
		s.loggedIn("test")
		s.localSigningSecret()
		s.env.Transport.Script(eventsRoute, eventsPage("",
			event("evt_01JD8M", "learner.certified", "12:00:00", "compliant"),
			event("evt_01JD8N", "learner.expired", "12:00:04", "expired")))
		s.forwardsTo(testkit.Reply{Status: 200, Before: s.takes(9)})
	},
	"print-secret": func(s *docSession) { s.localSigningSecret() },
	"trigger-test": func(s *docSession) {
		s.loggedIn("test")
		s.env.Transport.Script("POST "+orgPath+"/webhook-subscriptions/"+testkit.SubscriptionID+":test",
			testkit.Reply{Status: 202, Body: `{"name":"organizations/org_acme/operations/op_01JD86","done":true,` +
				`"metadata":{"kind":"webhookSubscriptions.test"},"response":{"_tag":"sent","status":204}}`})
	},
	"wait": func(s *docSession) {
		s.loggedIn("test")
		s.env.Transport.Script("POST "+orgPath+"/learners:export", accepted(`{"name":"`+operationName+`","done":true,`+
			`"metadata":{"kind":"learners.export"},"response":{"format":"csv","contentType":"text/csv",`+
			`"filename":"learners-2026-11-15.csv","rowCount":1,"dataBase64":"`+
			base64.StdEncoding.EncodeToString([]byte("email,name\r\nana@example.com,Ana Example\r\n"))+`"}}`))
	},
	"wait-failed": func(s *docSession) {
		s.loggedIn("test")
		s.env.Transport.Script(exportEnrollmentsRoute, accepted(failedOperation))
	},
	"export": func(s *docSession) {
		s.loggedIn("test")
		csv := "email,name,status\ndee@example.com,Dee Example,expired\neli@example.com,Eli Example,expired\n"
		s.env.Transport.Script("POST "+orgPath+"/learners:export", accepted(`{"name":"`+operationName+`","done":true,`+
			`"metadata":{"kind":"learners.export"},"response":{"format":"csv","contentType":"text/csv",`+
			`"filename":"learners-2026-11-15.csv","rowCount":2,"dataBase64":"`+
			base64.StdEncoding.EncodeToString([]byte(csv))+`"}}`))
	},
	"download-pdf": func(s *docSession) {
		s.loggedIn("test")
		s.env.Transport.Script(pdfRoute, pdfReply("%PDF-1.7\n"+strings.Repeat("0", 18432-len("%PDF-1.7\n"))))
	},
	"import-preview": func(s *docSession) {
		s.loggedIn("test")
		s.writeFile("roster.csv", "email,name\nana@example.com,Ana Example\nnot-an-email,Ben Example\n")
		s.env.Transport.Script(importRoute, testkit.JSON(`{"totalRows":2,"validRows":1,"seatsRequired":1,`+
			`"seatsAvailable":12,"rows":[{"rowNumber":1,"email":"ana@example.com","name":"Ana Example",`+
			`"courseId":"hipaa-2026","status":"valid"},{"rowNumber":2,"email":"not-an-email","name":"Ben Example",`+
			`"courseId":"hipaa-2026","status":"invalid_email","detail":"Not an email address."}]}`))
	},
	"validation-error": func(s *docSession) {
		s.loggedIn("test")
		s.env.Transport.Script("POST "+orgPath+"/learners", docProblem(422, "req_01JD8Q",
			`{"code":"validation_failed","title":"Validation failed","status":422,`+
				`"detail":"The request has invalid fields.","errors":[{"path":"email","message":"Invalid email"}]}`))
	},
	"usage-error": func(*docSession) {},
	"connection-error": func(s *docSession) {
		s.loggedIn("test")
		down := testkit.Reply{Err: errors.New("dial tcp: lookup sandbox.api.tme.so: no such host")}
		s.env.Transport.Script("GET "+orgPath+"/compliance", down, down, down)
	},
}

const (
	orgPath           = "/v1/organizations/" + testkit.OrgID
	listLearnersRoute = "GET " + orgPath + "/learners"
	// localSecret is the local Signing secret the docs show.
	localSecret = "3q2+7wAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
)

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

// loggedIn is a machine with TMH_CLIENT_ID and TMH_CLIENT_SECRET set.
func (s *docSession) loggedIn(mode string) {
	s.env.Vars["TMH_CLIENT_ID"] = testkit.ClientID
	s.env.Vars["TMH_CLIENT_SECRET"] = testkit.ClientSecret
	s.scriptLogins(mode, 1)
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

// takes makes a reply arrive after ms milliseconds on the fake clock.
func (s *docSession) takes(ms int) func() {
	return func() { s.env.Clock.Advance(time.Duration(ms) * time.Millisecond) }
}

func (s *docSession) localSigningSecret() {
	if err := s.env.Keyring.Set("tmh", "sandbox:local-signing-secret", localSecret); err != nil {
		s.t.Fatal(err)
	}
}

// forwardsTo answers `--forward-to http://localhost:3000/webhooks/tmh`.
func (s *docSession) forwardsTo(replies ...testkit.Reply) {
	forward := testkit.NewTransport(s.t)
	forward.Script("POST /webhooks/tmh", replies...)
	s.env.ForwardTransport = forward
}

func docProblem(status int, requestID, body string) testkit.Reply {
	return testkit.Reply{
		Status: status,
		Header: map[string]string{"Content-Type": "application/problem+json", "X-Request-Id": requestID},
		Body:   body,
	}
}

func event(id, eventType, at, status string) string {
	occurredAt := "2026-11-15T" + at + ".000Z"
	return `{"eventId":"` + id + `","eventType":"` + eventType + `","orgId":"org_acme","occurredAt":"` + occurredAt +
		`","data":{"learnerId":"sub_01JD80AB2C3D4E5F6G7H8J9K0M","email":"ana@example.com","externalId":"hr-123",` +
		`"courseId":"hipaa-2026","status":"` + status + `","occurredAt":"` + occurredAt + `"}}`
}

func expiredLearnerPages(s *docSession) {
	s.loggedIn("test")
	learner := func(id, email string) string {
		return `{"learnerId":"sub_01JD80AB2C3D4E5F6G7H8J9K1` + id + `","email":"` + email + `","status":"expired"}`
	}
	s.env.Transport.Script(listLearnersRoute,
		testkit.JSON(`{"items":[`+learner("A", "dee@example.com")+`,`+learner("B", "eli@example.com")+`],"nextCursor":"eyJvZmZzZXQiOjJ9"}`),
		testkit.JSON(`{"items":[`+learner("C", "fay@example.com")+`],"nextCursor":null}`))
}
