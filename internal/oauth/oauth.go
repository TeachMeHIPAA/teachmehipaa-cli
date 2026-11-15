// Package oauth runs the browser half of a Partner app's consent flow: PKCE,
// the consent URL and a loopback server that receives the redirect.
package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// PKCE is an RFC 7636 S256 pair. The verifier stays in this process until
// the code exchange; only the challenge goes through the browser.
type PKCE struct {
	Verifier  string
	Challenge string
}

// NewPKCE returns a fresh random verifier and its challenge.
func NewPKCE() (PKCE, error) {
	verifier, err := randomToken()
	if err != nil {
		return PKCE{}, err
	}
	digest := sha256.Sum256([]byte(verifier))
	return PKCE{Verifier: verifier, Challenge: base64.RawURLEncoding.EncodeToString(digest[:])}, nil
}

// NewState binds the redirect to this login, so a forged redirect can't
// attach someone else's Grant (RFC 6749 §10.12).
func NewState() (string, error) { return randomToken() }

// randomToken is 32 random bytes, which encode to 43 base64url characters:
// the shortest verifier RFC 7636 allows, with the 256 bits it recommends.
func randomToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// Consent is what the consent URL asks an Organization admin for.
type Consent struct {
	ClientID    string
	RedirectURI string
	Scopes      []string
	State       string
	Challenge   string
}

// AuthorizeURL is base with the consent request's query.
func AuthorizeURL(base string, consent Consent) (string, error) {
	target, err := url.Parse(base)
	if err != nil || !target.IsAbs() {
		return "", fmt.Errorf("the consent URL %q is not an absolute URL", base)
	}
	query := url.Values{
		"response_type":         {"code"},
		"client_id":             {consent.ClientID},
		"redirect_uri":          {consent.RedirectURI},
		"scope":                 {strings.Join(consent.Scopes, " ")},
		"state":                 {consent.State},
		"code_challenge":        {consent.Challenge},
		"code_challenge_method": {"S256"},
	}
	// Encode writes spaces as +, which only form parsers read back.
	target.RawQuery = strings.ReplaceAll(query.Encode(), "+", "%20")
	return target.String(), nil
}

// ErrStateMismatch means the redirect didn't come from this login's consent
// request.
var ErrStateMismatch = errors.New("the consent redirect's state does not match this login; nothing was exchanged, run the login again")

// ConsentError is the error the consent page redirected with, such as
// `access_denied` when the admin declined.
type ConsentError struct {
	Code        string
	Description string
}

// Error is the OAuth error code and its description.
func (e *ConsentError) Error() string {
	message := "consent was not granted: " + e.Code
	if e.Description != "" {
		message += ": " + e.Description
	}
	return message
}

// Loopback receives the consent redirect on 127.0.0.1 (RFC 8252 §7.3).
type Loopback struct {
	listener net.Listener
	server   *http.Server
	state    string
	results  chan callback
}

type callback struct {
	code string
	err  error
}

// Listen binds 127.0.0.1 on port, or on a port the OS picks when it is 0,
// for the redirect of the consent request carrying state. It binds the IPv4
// literal rather than localhost, which may resolve to IPv6 only.
func Listen(port int, state string) (*Loopback, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return nil, fmt.Errorf("could not listen for the consent redirect on 127.0.0.1:%d: %w", port, err)
	}
	loopback := &Loopback{listener: listener, state: state, results: make(chan callback, 1)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /callback", loopback.handle)
	loopback.server = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = loopback.server.Serve(listener) }()
	return loopback, nil
}

// RedirectURI is where the consent page sends the browser back to.
func (l *Loopback) RedirectURI() string {
	return "http://" + l.listener.Addr().String() + "/callback"
}

// handle hands the first redirect to Wait; later ones are answered but
// ignored, since the login already has its answer.
func (l *Loopback) handle(w http.ResponseWriter, r *http.Request) {
	result := l.read(r.URL.Query())
	select {
	case l.results <- result:
	default:
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if result.err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, "tmh login failed: %v\n", result.err)
		return
	}
	_, _ = fmt.Fprintln(w, "tmh is logged in. You can close this tab and return to the terminal.")
}

func (l *Loopback) read(query url.Values) callback {
	if subtle.ConstantTimeCompare([]byte(query.Get("state")), []byte(l.state)) != 1 {
		return callback{err: ErrStateMismatch}
	}
	if code := query.Get("error"); code != "" {
		return callback{err: &ConsentError{Code: code, Description: query.Get("error_description")}}
	}
	if query.Get("code") == "" {
		return callback{err: errors.New("the consent redirect carried no code")}
	}
	return callback{code: query.Get("code")}
}

// Wait returns the authorization code from the first redirect. It waits
// until ctx ends.
func (l *Loopback) Wait(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case result := <-l.results:
		return result.code, result.err
	}
}

// Close stops listening, waiting at most a second for the browser's
// request to finish.
func (l *Loopback) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return l.server.Shutdown(ctx)
}
