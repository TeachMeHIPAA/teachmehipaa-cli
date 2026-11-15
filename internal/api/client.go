// Package api is tmh's HTTP client for the TeachMeHIPAA Developer API. It
// follows the same rules as the SDKs: client-credentials Access tokens, the
// Organization bound via introspection (ADR-0003), problem+json errors and a
// content-type guard. It never logs bodies (ADR-0007).
package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/clock"
)

// Config is what a Client needs: where the API is, whose credentials to
// use, and how to send, trace and retry requests.
type Config struct {
	// BaseURL ends in /v1, without a trailing slash.
	BaseURL      string
	ClientID     string
	ClientSecret string
	// OrganizationID skips the introspection lookup when already known.
	OrganizationID string
	Transport      http.RoundTripper
	Clock          clock.Clock
	UserAgent      string
	// Timeout bounds each HTTP exchange.
	Timeout time.Duration
	// MaxRetries is how many times a transient failure is retried.
	MaxRetries int
	// Trace, when set, sees every HTTP attempt, retries and token mints
	// included.
	Trace func(Exchange)
	// Grant, when set, makes ClientID and ClientSecret a Partner app's and
	// mints Access tokens from the Grant's Refresh token instead of the
	// client-credentials grant.
	Grant GrantStore
}

// Client lives for one command, so its Access token is cached only in memory.
type Client struct {
	config         Config
	http           *http.Client
	token          string
	tokenExpiresAt time.Time
	organizationID string
}

// New returns a Client that has not yet sent anything; it mints an Access
// token on its first request.
func New(config Config) *Client {
	return &Client{
		config:         config,
		http:           &http.Client{Transport: config.Transport, Timeout: config.Timeout},
		organizationID: config.OrganizationID,
	}
}

// Request is one API call. Path is relative to the base URL with path
// parameters already escaped.
type Request struct {
	Method string
	Path   string
	// OrganizationScoped prefixes Path with /organizations/{orgId}.
	OrganizationScoped bool
	Query              url.Values
	// RetrySafe means the request may be replayed after a transient
	// failure, and is the only thing that makes it retryable: the IR's
	// `retrySafe` for a generated command, the caller's own judgment
	// otherwise.
	RetrySafe bool
	// IdempotencyKey is sent as the Idempotency-Key header when set. It is
	// fixed per call, so every retry carries the same key.
	IdempotencyKey string
	// Body is sent with ContentType when non-nil, unchanged on every retry.
	Body        []byte
	ContentType string
	// IfMatch is sent as the If-Match header when set.
	IfMatch string
	// Public is the IR's `security: none`: no Access token is minted or
	// sent, so the operation works without credentials.
	Public bool
	// Accept, when set, asks for a file instead of JSON, such as
	// "application/pdf, application/problem+json" or "*/*". A success must
	// then have one of its media types other than problem+json.
	Accept string
}

// Response is a successful API response whose body is JSON, or empty for 204.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// Exchange is one HTTP attempt as --verbose reports it. It holds no bodies
// and no request headers, so it can't carry a credential.
type Exchange struct {
	Method string
	// Path has no query string, which may hold personal data.
	Path string
	// Status is 0 when no response arrived.
	Status         int
	Latency        time.Duration
	RequestID      string
	IdempotencyKey string
	// Header is the response's.
	Header http.Header
}

// ConnectionError means no HTTP response arrived.
type ConnectionError struct{ Err error }

// Error names the API as what could not be reached.
func (e *ConnectionError) Error() string {
	return fmt.Sprintf("could not reach the TeachMeHIPAA API: %v", e.Err)
}

// Unwrap returns the network error.
func (e *ConnectionError) Unwrap() error { return e.Err }

// Do sends request, retrying transient failures when it is retry-safe. A 401
// `unauthorized` gets one fresh Access token and a replay that doesn't count
// as a retry: the API rejected it before acting, so even a PATCH is safe.
func (c *Client) Do(ctx context.Context, request Request) (*Response, error) {
	path := request.Path
	if request.OrganizationScoped {
		organizationID, err := c.OrganizationID(ctx)
		if err != nil {
			return nil, err
		}
		path = "/organizations/" + url.PathEscape(organizationID) + path
	}
	target := c.config.BaseURL + path
	if len(request.Query) > 0 {
		target += "?" + request.Query.Encode()
	}
	replayed := request.Public
	for retry := 1; ; retry++ {
		var token string
		if !request.Public {
			var err error
			if token, err = c.accessToken(ctx); err != nil {
				return nil, err
			}
		}
		response, err := c.exchange(ctx, request, target, token)
		if !replayed && isUnauthorized(err) {
			replayed = true
			c.token = ""
			if token, err = c.accessToken(ctx); err != nil {
				return nil, err
			}
			response, err = c.exchange(ctx, request, target, token)
		}
		if err == nil {
			return response, nil
		}
		if !request.RetrySafe || !c.retryAfterWait(ctx, err, retry, isTransient) {
			return nil, err
		}
	}
}

func (c *Client) exchange(ctx context.Context, request Request, target, token string) (*Response, error) {
	var requestBody io.Reader
	if request.Body != nil {
		requestBody = bytes.NewReader(request.Body)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, request.Method, target, requestBody)
	if err != nil {
		return nil, err
	}
	if request.Body != nil {
		httpRequest.Header.Set("Content-Type", request.ContentType)
	}
	if request.IfMatch != "" {
		httpRequest.Header.Set("If-Match", request.IfMatch)
	}
	if token != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+token)
	}
	accept := request.Accept
	if accept == "" {
		accept = "application/json"
	}
	httpRequest.Header.Set("Accept", accept)
	httpRequest.Header.Set("User-Agent", c.config.UserAgent)
	if request.IdempotencyKey != "" {
		httpRequest.Header.Set("Idempotency-Key", request.IdempotencyKey)
	}
	status, header, body, err := c.send(httpRequest)
	if err != nil {
		return nil, err
	}
	switch {
	case status < 200 || status > 299:
		return nil, problemError(status, header, body)
	case status == http.StatusNoContent || (request.Accept == "" && isJSON(header)):
	case request.Accept == "":
		return nil, problemError(status, header, body)
	case !accepts(request.Accept, header):
		return nil, unexpectedType(status, header, request.Accept)
	}
	return &Response{Status: status, Header: header, Body: body}, nil
}

// retryAfterWait reports whether failed attempt number `retry` should be
// retried, after waiting out the delay.
func (c *Client) retryAfterWait(ctx context.Context, err error, retry int, retryable func(error) bool) bool {
	if retry > c.config.MaxRetries || !retryable(err) || ctx.Err() != nil {
		return false
	}
	return c.config.Clock.Sleep(ctx, retryDelay(err, retry, c.config.Clock.Now())) == nil
}

// Introspection is `TokenIntrospection`: what the Access token can do.
type Introspection struct {
	ActorType string   `json:"actorType"`
	ClientID  *string  `json:"clientId"`
	OrgID     *string  `json:"orgId"`
	Scopes    []string `json:"scopes"`
	// Mode is "test" for a Sandbox client and "live" otherwise.
	Mode      *string `json:"mode"`
	ExpiresAt *string `json:"expiresAt"`
}

// Introspect asks the API what the client's Access token is bound to.
func (c *Client) Introspect(ctx context.Context) (*Introspection, error) {
	response, err := c.Do(ctx, Request{Method: http.MethodGet, Path: "/auth/introspection", RetrySafe: true})
	if err != nil {
		return nil, err
	}
	var introspection Introspection
	if err := json.Unmarshal(response.Body, &introspection); err != nil {
		return nil, fmt.Errorf("reading the token introspection: %w", err)
	}
	return &introspection, nil
}

// OrganizationID is the Organization the credentials are bound to, looked up
// at most once.
func (c *Client) OrganizationID(ctx context.Context) (string, error) {
	if c.organizationID != "" {
		return c.organizationID, nil
	}
	introspection, err := c.Introspect(ctx)
	if err != nil {
		return "", err
	}
	if introspection.OrgID == nil || *introspection.OrgID == "" {
		return "", errors.New("these credentials are not bound to an Organization")
	}
	c.organizationID = *introspection.OrgID
	return c.organizationID, nil
}

// Expiry is counted from when the mint request left, and the token is
// replaced this long before then so it never expires mid-request.
const remintMargin = 60 * time.Second

func (c *Client) accessToken(ctx context.Context) (string, error) {
	if c.token != "" && c.config.Clock.Now().Before(c.tokenExpiresAt.Add(-remintMargin)) {
		return c.token, nil
	}
	if c.config.Grant != nil {
		if err := c.refresh(ctx); err != nil {
			return "", err
		}
		return c.token, nil
	}
	// Each mint returns a fresh token, so unlike a Refresh token exchange
	// (ADR-0004) one whose outcome is unknown may be repeated.
	for retry := 1; ; retry++ {
		err := c.mint(ctx)
		if err == nil {
			return c.token, nil
		}
		if !c.retryAfterWait(ctx, err, retry, isMintRetryable) {
			return "", err
		}
	}
}

func (c *Client) mint(ctx context.Context) error {
	requestedAt := c.config.Clock.Now()
	status, header, body, err := c.postToken(ctx, url.Values{"grant_type": {"client_credentials"}})
	if err != nil {
		return err
	}
	token, failure := parseTokenResponse(status, header, body)
	if failure != nil {
		return failure
	}
	c.useToken(token.AccessToken, requestedAt, *token.ExpiresIn)
	return nil
}

// tokenResponse is what every grant type answers with; only the
// `authorization_code` and `refresh_token` grants include RefreshToken.
type tokenResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    *float64 `json:"expires_in"`
}

// parseTokenResponse reads a successful token response, or the OAuth error
// that answered instead.
func parseTokenResponse(status int, header http.Header, body []byte) (tokenResponse, *Error) {
	var token tokenResponse
	if status != http.StatusOK || !isJSON(header) ||
		json.Unmarshal(body, &token) != nil || token.AccessToken == "" || token.ExpiresIn == nil {
		return tokenResponse{}, oauthError(status, header, body)
	}
	return token, nil
}

// useToken caches an Access token whose expiry counts from requestedAt.
func (c *Client) useToken(accessToken string, requestedAt time.Time, expiresIn float64) {
	c.token = accessToken
	c.tokenExpiresAt = requestedAt.Add(time.Duration(expiresIn * float64(time.Second)))
}

// postToken sends one form to the token endpoint, authenticated with the
// client's id and secret.
func (c *Client) postToken(ctx context.Context, form url.Values) (int, http.Header, []byte, error) {
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.config.BaseURL+"/auth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return 0, nil, nil, err
	}
	// RFC 6749 §2.3.1: Basic credentials are form-encoded before base64.
	credentials := url.QueryEscape(c.config.ClientID) + ":" + url.QueryEscape(c.config.ClientSecret)
	httpRequest.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(credentials)))
	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("User-Agent", c.config.UserAgent)
	return c.send(httpRequest)
}

func (c *Client) send(request *http.Request) (int, http.Header, []byte, error) {
	startedAt := c.config.Clock.Now()
	status, header, body, err := c.roundTrip(request)
	if c.config.Trace != nil {
		c.config.Trace(Exchange{
			Method:         request.Method,
			Path:           request.URL.Path,
			Status:         status,
			Latency:        c.config.Clock.Now().Sub(startedAt),
			RequestID:      header.Get("X-Request-Id"),
			IdempotencyKey: request.Header.Get("Idempotency-Key"),
			Header:         header,
		})
	}
	return status, header, body, err
}

func (c *Client) roundTrip(request *http.Request) (int, http.Header, []byte, error) {
	response, err := c.http.Do(request)
	if err != nil {
		return 0, nil, nil, &ConnectionError{Err: err}
	}
	defer response.Body.Close()
	var body bytes.Buffer
	if _, err := io.Copy(&body, response.Body); err != nil {
		return 0, nil, nil, &ConnectionError{Err: err}
	}
	return response.StatusCode, response.Header, body.Bytes(), nil
}
