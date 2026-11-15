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
}

// Client lives for one command, so its Access token is cached only in memory.
type Client struct {
	config         Config
	http           *http.Client
	token          string
	tokenExpiresAt time.Time
	organizationID string
}

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
}

// Response is a successful API response whose body is known to be JSON.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// ConnectionError means no HTTP response arrived.
type ConnectionError struct{ Err error }

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("could not reach the TeachMeHIPAA API: %v", e.Err)
}

func (e *ConnectionError) Unwrap() error { return e.Err }

func (c *Client) Do(ctx context.Context, request Request) (*Response, error) {
	path := request.Path
	if request.OrganizationScoped {
		organizationID, err := c.OrganizationID(ctx)
		if err != nil {
			return nil, err
		}
		path = "/organizations/" + url.PathEscape(organizationID) + path
	}
	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	target := c.config.BaseURL + path
	if len(request.Query) > 0 {
		target += "?" + request.Query.Encode()
	}
	httpRequest, err := http.NewRequestWithContext(ctx, request.Method, target, nil)
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+token)
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("User-Agent", c.config.UserAgent)

	status, header, body, err := c.send(httpRequest)
	if err != nil {
		return nil, err
	}
	if status < 200 || status > 299 || !isJSON(header) {
		return nil, problemError(status, header, body)
	}
	return &Response{Status: status, Header: header, Body: body}, nil
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

func (c *Client) Introspect(ctx context.Context) (*Introspection, error) {
	response, err := c.Do(ctx, Request{Method: http.MethodGet, Path: "/auth/introspection"})
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
	requestedAt := c.config.Clock.Now()
	form := url.Values{"grant_type": {"client_credentials"}}.Encode()
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.config.BaseURL+"/auth/token", strings.NewReader(form))
	if err != nil {
		return "", err
	}
	// RFC 6749 §2.3.1: Basic credentials are form-encoded before base64.
	credentials := url.QueryEscape(c.config.ClientID) + ":" + url.QueryEscape(c.config.ClientSecret)
	httpRequest.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(credentials)))
	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("User-Agent", c.config.UserAgent)

	status, header, body, err := c.send(httpRequest)
	if err != nil {
		return "", err
	}
	var token struct {
		AccessToken string   `json:"access_token"`
		ExpiresIn   *float64 `json:"expires_in"`
	}
	if status != http.StatusOK || !isJSON(header) ||
		json.Unmarshal(body, &token) != nil || token.AccessToken == "" || token.ExpiresIn == nil {
		return "", oauthError(status, header, body)
	}
	c.token = token.AccessToken
	c.tokenExpiresAt = requestedAt.Add(time.Duration(*token.ExpiresIn * float64(time.Second)))
	return c.token, nil
}

func (c *Client) send(request *http.Request) (int, http.Header, []byte, error) {
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
