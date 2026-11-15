package events_test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/events"
)

// vector is the shared Delivery vector, signed by the platform's own signer
// (testdata/webhook-vector.json, copied from the SDK program's _shared dir).
type vector struct {
	Secret string `json:"secret"`
	Path   string `json:"path"`
	Event  struct {
		EventType string            `json:"eventType"`
		EventID   string            `json:"eventId"`
		Timestamp string            `json:"timestamp"`
		Body      string            `json:"body"`
		Signature string            `json:"signature"`
		Headers   map[string]string `json:"headers"`
	} `json:"event"`
}

func loadVector(t *testing.T) vector {
	t.Helper()
	raw, err := os.ReadFile("testdata/webhook-vector.json")
	if err != nil {
		t.Fatal(err)
	}
	var v vector
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestTheLocalSignatureMatchesThePlatformsVector(t *testing.T) {
	v := loadVector(t)
	at, err := time.Parse(time.RFC3339Nano, v.Event.Timestamp)
	if err != nil {
		t.Fatal(err)
	}

	headers := events.Sign(v.Secret, v.Path, at, events.Event{
		ID:   v.Event.EventID,
		Type: v.Event.EventType,
		Body: []byte(v.Event.Body),
	})

	for name, want := range v.Event.Headers {
		if name == "x-tmh-key-id" {
			want = "local"
		}
		if got := headers.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestTheForwardedBodyIsThePlatformsCanonicalJSON(t *testing.T) {
	v := loadVector(t)
	// The vector's envelope as an API page would list it: keys in wire
	// order, with whitespace.
	listed := `{
	  "eventId": "evt_01JCZ8Y2K5V3M7Q9R4T6W8X0AB",
	  "eventType": "learner.certified",
	  "orgId": "org_9f3k2m",
	  "occurredAt": "2026-11-15T16:31:30.000Z",
	  "data": {"learnerId": "lrn_4h7j2k", "email": "zoë.müller@example.com",
	    "externalId": "hr-123", "courseId": "hipaa-basics-2026", "status": "certified",
	    "occurredAt": "2026-11-15T16:31:30.000Z"}
	}`

	body, err := events.Canonicalize([]byte(listed))

	if err != nil || string(body) != v.Event.Body {
		t.Errorf("Canonicalize() = %s, %v\nwant %s", body, err, v.Event.Body)
	}
}

func TestCanonicalJSONFollowsJavaScriptsKeyOrderAndFormatting(t *testing.T) {
	listed := `{"z":{"b":[3,{"y":1,"x":2}],"a":null},"10":"ten","9":"nine",` +
		`"b":"<tag>&amp; \u0007\b\f/","A":1e21,"é":true,"ｚ":1,"😀":2,` +
		`"n":1.50,"m":1e-7,"k":-0,"big":12345678901234567890}`
	// Printed by the platform's canonicalizeEventBody(JSON.parse(listed)):
	// array-index keys come first, then UTF-16 order, and numbers and
	// strings are formatted as JSON.stringify does.
	want := "{\"9\":\"nine\",\"10\":\"ten\",\"A\":1e+21,\"b\":\"<tag>&amp; \\u0007\\b\\f/\",\"big\":12345678901234567000,\"k\":0,\"m\":1e-7,\"n\":1.5,\"z\":{\"a\":null,\"b\":[3,{\"x\":2,\"y\":1}]},\"é\":true,\"\U0001F600\":2,\"ｚ\":1}"

	body, err := events.Canonicalize([]byte(listed))

	if err != nil || string(body) != want {
		t.Errorf("Canonicalize() = %s, %v\nwant %s", body, err, want)
	}
}
