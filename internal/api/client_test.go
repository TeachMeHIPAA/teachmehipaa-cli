package api_test

import (
	"context"
	"testing"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/api"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/testkit"
)

const (
	baseURL      = "https://api-teachmehipaa-dev.web.app/v1"
	learnersPath = "/organizations/" + testkit.OrgID + "/learners"
)

// TestOnlyRetrySafeMakesARequestRetryable covers what no IR method has yet:
// the HTTP method and an Idempotency-Key never make a request retryable on
// their own.
func TestOnlyRetrySafeMakesARequestRetryable(t *testing.T) {
	tests := []struct {
		name         string
		request      api.Request
		wantRequests int
	}{
		{"a POST with a key but not RetrySafe", api.Request{Method: "POST", Path: learnersPath, IdempotencyKey: "sync-1"}, 1},
		{"a PUT with a key but not RetrySafe", api.Request{Method: "PUT", Path: learnersPath, IdempotencyKey: "sync-1"}, 1},
		{"a GET that is not RetrySafe", api.Request{Method: "GET", Path: learnersPath}, 1},
		{"a RetrySafe POST", api.Request{Method: "POST", Path: learnersPath, RetrySafe: true}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := tt.request.Method + " /v1" + learnersPath
			transport := testkit.NewTransport(t).
				Script(testkit.TokenRoute, testkit.TokenReply("jwt-1", 900)).
				Script(route, testkit.Reply{Status: 503, Header: map[string]string{"Content-Type": "application/problem+json"}, Body: `{"code":"unavailable"}`}, testkit.JSON(`{}`))
			client := api.New(api.Config{
				BaseURL: baseURL, ClientID: testkit.ClientID, ClientSecret: testkit.ClientSecret,
				Transport: transport, Clock: &testkit.Clock{}, MaxRetries: 2,
			})

			_, err := client.Do(context.Background(), tt.request)

			n := 0
			for _, r := range transport.Routes() {
				if r == route {
					n++
				}
			}
			if n != tt.wantRequests {
				t.Errorf("requests = %d, want %d", n, tt.wantRequests)
			}
			if (err == nil) != (tt.wantRequests == 2) {
				t.Errorf("err = %v", err)
			}
		})
	}
}
