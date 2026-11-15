package api

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"strings"
)

// FieldError is one invalid field from a `validation_failed` problem.
type FieldError struct {
	// Path is dot-joined, or `Idempotency-Key` for a missing header.
	Path    string `json:"path"`
	Message string `json:"message"`
}

// Error is an API failure: a problem+json body, an OAuth error from the token
// endpoint, or a response that wasn't JSON at all.
type Error struct {
	Status int
	// Code is the machine-readable reason, such as `seat_unavailable`, or the
	// OAuth `error`. New codes may appear at any time.
	Code   string
	Title  string
	Detail string
	Errors []FieldError
	// RequestID is what TeachMeHIPAA support asks for.
	RequestID string
	// OAuth marks a failure from the token endpoint.
	OAuth   bool
	message string
}

func (e *Error) Error() string { return e.message }

// isJSON guards every parse: unknown paths answer 200 with an HTML page.
func isJSON(header http.Header) bool {
	mediaType, _, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil {
		return false
	}
	return mediaType == "application/json" ||
		(strings.HasPrefix(mediaType, "application/") && strings.HasSuffix(mediaType, "+json"))
}

func notJSONMessage(status int, header http.Header) string {
	contentType := header.Get("Content-Type")
	if contentType == "" {
		contentType = "no content type"
	}
	return fmt.Sprintf("expected a JSON response but got %s with HTTP %d; check the base URL", contentType, status)
}

func problemError(status int, header http.Header, body []byte) *Error {
	e := &Error{Status: status, RequestID: header.Get("X-Request-Id")}
	var problem struct {
		Code   string       `json:"code"`
		Title  string       `json:"title"`
		Detail string       `json:"detail"`
		Errors []FieldError `json:"errors"`
	}
	if !isJSON(header) || json.Unmarshal(body, &problem) != nil {
		e.message = notJSONMessage(status, header)
		return e
	}
	e.Code, e.Title, e.Detail, e.Errors = problem.Code, problem.Title, problem.Detail, problem.Errors
	e.message = joinNonEmpty(problem.Title, problem.Detail)
	if e.message == "" {
		e.message = fmt.Sprintf("the TeachMeHIPAA API responded with HTTP %d", status)
	}
	return e
}

func oauthError(status int, header http.Header, body []byte) *Error {
	e := &Error{Status: status, RequestID: header.Get("X-Request-Id"), OAuth: true}
	var oauth struct {
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	if !isJSON(header) || json.Unmarshal(body, &oauth) != nil || oauth.Error == "" {
		e.message = notJSONMessage(status, header)
		if isJSON(header) {
			e.message = fmt.Sprintf("the token endpoint responded with HTTP %d", status)
		}
		return e
	}
	e.Code, e.Detail = oauth.Error, oauth.Description
	e.message = joinNonEmpty(oauth.Error, oauth.Description)
	return e
}

func joinNonEmpty(parts ...string) string {
	kept := []string{}
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, ": ")
}
