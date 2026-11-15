package events

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/clock"
)

// Event is one item of the Organization's event log.
type Event struct {
	ID         string
	Type       string
	OccurredAt string
	// Body is the Event as a Delivery carries it.
	Body []byte
}

// Parse reads an Event as the event log lists it. Event types the IR doesn't
// know yet are kept like any other (ADR-0006).
func Parse(raw json.RawMessage) (Event, error) {
	var envelope struct {
		EventID    string `json:"eventId"`
		EventType  string `json:"eventType"`
		OccurredAt string `json:"occurredAt"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return Event{}, fmt.Errorf("reading an Event: %w", err)
	}
	if envelope.EventID == "" {
		return Event{}, errors.New("an Event in the log has no eventId")
	}
	body, err := Canonicalize(raw)
	if err != nil {
		return Event{}, fmt.Errorf("reading Event %s: %w", envelope.EventID, err)
	}
	return Event{ID: envelope.EventID, Type: envelope.EventType, OccurredAt: envelope.OccurredAt, Body: body}, nil
}

// forwardTimeout keeps a hung local handler from stalling the tail forever.
const forwardTimeout = 30 * time.Second

// Forwarder POSTs Events to the developer's local endpoint.
type Forwarder struct {
	target *url.URL
	secret string
	clock  clock.Clock
	client *http.Client
}

// ParseTarget accepts an absolute http or https URL.
func ParseTarget(raw string) (*url.URL, error) {
	target, err := url.Parse(raw)
	if err != nil || (target.Scheme != "http" && target.Scheme != "https") || target.Host == "" {
		return nil, fmt.Errorf("--forward-to must be an http:// or https:// URL, such as http://localhost:3000/webhooks; got %q", raw)
	}
	return target, nil
}

// NewForwarder sends through transport, which must not be the API's: the
// target is the developer's server and gets nothing but the Delivery.
func NewForwarder(target *url.URL, secret string, c clock.Clock, transport http.RoundTripper) *Forwarder {
	return &Forwarder{
		target: target,
		secret: secret,
		clock:  c,
		client: &http.Client{
			Transport: transport,
			Timeout:   forwardTimeout,
			// A redirect would re-send the Delivery to a path its signature
			// doesn't cover, so the developer sees the 3xx instead.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

// Target is the URL Events are forwarded to.
func (f *Forwarder) Target() string { return f.target.String() }

// Result is how the target answered one Event.
type Result struct {
	// Status is 0 when no response arrived.
	Status  int
	Latency time.Duration
	Err     error
}

// OK means the target accepted the Event with a 2xx.
func (r Result) OK() bool { return r.Err == nil && r.Status >= 200 && r.Status <= 299 }

// Forward signs the Event for the target's path and query and sends it once.
func (f *Forwarder) Forward(ctx context.Context, event Event) Result {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, f.target.String(), bytes.NewReader(event.Body))
	if err != nil {
		return Result{Err: err}
	}
	startedAt := f.clock.Now()
	request.Header = Sign(f.secret, pathAndQuery(f.target), startedAt, event.Body,
		Meta{EventID: event.ID, EventType: event.Type})
	response, err := f.client.Do(request)
	latency := f.clock.Now().Sub(startedAt)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return Result{Latency: latency, Err: err}
	}
	defer response.Body.Close()
	// Draining lets the connection be reused; the body itself is the
	// developer's and is never printed (ADR-0007).
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
	return Result{Status: response.StatusCode, Latency: latency}
}

// pathAndQuery is what the request line carries, which is what the
// receiving SDK verifies against.
func pathAndQuery(target *url.URL) string {
	path := target.EscapedPath()
	if path == "" {
		path = "/"
	}
	if target.RawQuery != "" {
		path += "?" + target.RawQuery
	}
	return path
}
