// Package testkit runs tmh commands against fakes: a scripted HTTP transport,
// an in-memory keyring, a settable clock and captured stdio.
package testkit

import (
	"bytes"
	"io"
	"net/http"
	"sync"
	"testing"
)

// Reply is one scripted HTTP response.
type Reply struct {
	Status int
	// Header values; content-type defaults to application/json.
	Header map[string]string
	Body   string
	// Before runs when the request arrives, e.g. to advance the clock.
	Before func()
	// Err fails the request with no response, like a refused connection.
	Err error
}

// JSON scripts a 200 JSON reply.
func JSON(body string) Reply { return Reply{Status: http.StatusOK, Body: body} }

// Request is what the fake transport saw.
type Request struct {
	Method string
	URL    string
	Path   string
	Header http.Header
	Body   string
}

// Transport replays scripted replies per route ("GET /v1/auth/introspection")
// in order and records every request.
type Transport struct {
	t        testing.TB
	mu       sync.Mutex
	routes   map[string][]Reply
	requests []Request
}

func NewTransport(t testing.TB) *Transport {
	return &Transport{t: t, routes: map[string][]Reply{}}
}

// Script queues replies for a route, consumed one per request.
func (f *Transport) Script(route string, replies ...Reply) *Transport {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.routes[route] = append(f.routes[route], replies...)
	return f
}

// Requests returns every request in arrival order.
func (f *Transport) Requests() []Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Request(nil), f.requests...)
}

// Routes returns "METHOD /path" for every request in arrival order.
func (f *Transport) Routes() []string {
	routes := []string{}
	for _, r := range f.Requests() {
		routes = append(routes, r.Method+" "+r.Path)
	}
	return routes
}

func (f *Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	body := ""
	if r.Body != nil {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		body = string(raw)
	}
	route := r.Method + " " + r.URL.Path

	f.mu.Lock()
	f.requests = append(f.requests, Request{
		Method: r.Method,
		URL:    r.URL.String(),
		Path:   r.URL.Path,
		Header: r.Header.Clone(),
		Body:   body,
	})
	queue := f.routes[route]
	if len(queue) == 0 {
		f.mu.Unlock()
		f.t.Errorf("unscripted request: %s", route)
		return reply(r, Reply{Status: http.StatusTeapot, Body: `{}`}), nil
	}
	next := queue[0]
	f.routes[route] = queue[1:]
	f.mu.Unlock()

	if next.Before != nil {
		next.Before()
	}
	if next.Err != nil {
		return nil, next.Err
	}
	return reply(r, next), nil
}

func reply(r *http.Request, scripted Reply) *http.Response {
	header := http.Header{"Content-Type": []string{"application/json"}}
	for name, value := range scripted.Header {
		header.Set(name, value)
	}
	status := scripted.Status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		Status:        http.StatusText(status),
		StatusCode:    status,
		Header:        header,
		Body:          io.NopCloser(bytes.NewBufferString(scripted.Body)),
		ContentLength: int64(len(scripted.Body)),
		Request:       r,
	}
}
