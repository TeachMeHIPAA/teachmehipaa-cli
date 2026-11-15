package api

import (
	"encoding/json"
	"errors"
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
	Header    http.Header
	// OAuth marks an RFC 6749 error body from the token endpoint.
	OAuth bool
	// UnexpectedBody means the body wasn't JSON, such as the HTML page a
	// wrong base URL answers with.
	UnexpectedBody bool
	message        string
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

// accepts reports whether a success's content type is one the Accept header
// asked for. problem+json only ever describes a failure.
func accepts(accept string, header http.Header) bool {
	mediaType, _, err := mime.ParseMediaType(header.Get("Content-Type"))
	for _, wanted := range strings.Split(accept, ",") {
		wanted, _, _ = strings.Cut(strings.TrimSpace(wanted), ";")
		switch {
		case wanted == "*/*":
			return true
		case err == nil && wanted == mediaType && wanted != "application/problem+json":
			return true
		}
	}
	return false
}

// unexpectedType is a success in a format that wasn't asked for, such as
// the HTML page a wrong base URL answers a download with.
func unexpectedType(status int, header http.Header, accept string) *Error {
	contentType := header.Get("Content-Type")
	if contentType == "" {
		contentType = "no content type"
	}
	wanted, _, _ := strings.Cut(accept, ",")
	return &Error{
		Status:         status,
		RequestID:      header.Get("X-Request-Id"),
		Header:         header,
		UnexpectedBody: true,
		message:        fmt.Sprintf("expected %s but got %s with HTTP %d; check the base URL", wanted, contentType, status),
	}
}

func problemError(status int, header http.Header, body []byte) *Error {
	e := &Error{Status: status, RequestID: header.Get("X-Request-Id"), Header: header}
	var problem struct {
		Code   string       `json:"code"`
		Title  string       `json:"title"`
		Detail string       `json:"detail"`
		Errors []FieldError `json:"errors"`
	}
	if !isJSON(header) || json.Unmarshal(body, &problem) != nil {
		e.UnexpectedBody = true
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

// oauthError reads the token endpoint's `{error, error_description}`; any
// other body from it, such as a gateway's 502 page, maps like any response.
func oauthError(status int, header http.Header, body []byte) *Error {
	var oauth struct {
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	if !isJSON(header) || json.Unmarshal(body, &oauth) != nil || oauth.Error == "" {
		return problemError(status, header, body)
	}
	return &Error{
		Status:    status,
		Code:      oauth.Error,
		Detail:    oauth.Description,
		RequestID: header.Get("X-Request-Id"),
		Header:    header,
		OAuth:     true,
		message:   joinNonEmpty(oauth.Error, oauth.Description),
	}
}

// isTransient is a failure that left no trace on the server, or one the
// server asks us to retry.
func isTransient(err error) bool {
	var connectionErr *ConnectionError
	if errors.As(err, &connectionErr) {
		return true
	}
	var apiErr *Error
	return errors.As(err, &apiErr) && (apiErr.Status == http.StatusTooManyRequests ||
		(apiErr.Status == http.StatusConflict && apiErr.Code == "conflict") ||
		apiErr.Status >= 500)
}

// isMintRetryable leaves out 4xx answers such as `invalid_client`: asking
// again with the same credentials gets the same answer.
func isMintRetryable(err error) bool {
	var connectionErr *ConnectionError
	if errors.As(err, &connectionErr) {
		return true
	}
	var apiErr *Error
	return errors.As(err, &apiErr) && apiErr.Status >= 500 && apiErr.Code != "invalid_client"
}

func isUnauthorized(err error) bool {
	var apiErr *Error
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusUnauthorized && apiErr.Code == "unauthorized"
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
