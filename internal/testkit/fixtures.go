package testkit

import "fmt"

// Fixture values follow the developer OpenAPI examples (api-sdk-surface.md §2).
const (
	ClientID     = "cli_01JD7Z3M1V4Q8T2K5N9R6W0XYZ"
	ClientSecret = "tmh_cs_c2VjcmV0LXNlY3JldC1zZWNyZXQtc2VjcmV0LXM"
	OrgID        = "org_acme"
	LearnerID    = "sub_01JD80AB2C3D4E5F6G7H8J9K0M"

	TokenRoute         = "POST /v1/auth/token"
	IntrospectionRoute = "GET /v1/auth/introspection"
	LearnerRoute       = "GET /v1/organizations/" + OrgID + "/learners/" + LearnerID

	EnrollmentID                   = "enr_01JD81CD2E3F4G5H6J7K8M9N0P"
	SubscriptionID                 = "whs_01JD82EF3G4H5J6K7M8N9P0Q1R"
	RemindRoute                    = "POST /v1/organizations/" + OrgID + "/enrollments/" + EnrollmentID + ":remind"
	LaunchLinkRoute                = "POST /v1/organizations/" + OrgID + "/learners/" + LearnerID + ":createLaunchLink"
	DeleteWebhookSubscriptionRoute = "DELETE /v1/organizations/" + OrgID + "/webhook-subscriptions/" + SubscriptionID
	RemindAcceptedBody             = `{"enrollmentId":"` + EnrollmentID + `","action":"remind"}`
	LaunchLinkBody                 = `{"url":"https://training.teachmehipaa.com/launch/abc","expiresAt":"2026-11-15T12:05:00.000Z"}`
)

// TokenReply is `ClientCredentialsTokenResponse`.
func TokenReply(accessToken string, expiresIn int) Reply {
	return Reply{
		Body: fmt.Sprintf(`{"access_token":%q,"token_type":"Bearer","expires_in":%d,"scope":"learners:read learners:write"}`,
			accessToken, expiresIn),
		Header: map[string]string{"Cache-Control": "no-store", "Pragma": "no-cache"},
	}
}

// IntrospectionReply is `TokenIntrospection` for an API client.
func IntrospectionReply(mode string) Reply {
	return JSON(fmt.Sprintf(`{"actorType":"client","clientId":%q,"subjectId":null,"orgId":%q,"scopes":["learners:read","learners:write"],"mode":%q,"expiresAt":"2026-11-15T12:15:00.000Z"}`,
		ClientID, OrgID, mode))
}

// LearnerBody is `LearnerDto`, compact and in wire order.
const LearnerBody = `{"learnerId":"` + LearnerID + `","email":"ana@example.com","name":"Ana Example","externalId":"hr-123","status":"in_progress","course":{"courseId":"hipaa-2026","title":"HIPAA Training 2026"},"invitedAt":"2026-11-01T09:00:00.000Z","startedAt":"2026-11-02T10:30:00.000Z","completedAt":null,"dueAt":"2026-12-01T00:00:00.000Z","certificate":null}`
