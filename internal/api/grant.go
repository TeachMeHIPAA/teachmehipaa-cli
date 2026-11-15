package api

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// GrantStore keeps a Partner app Grant's Refresh token between commands.
type GrantStore interface {
	// Lock keeps every other tmh process from exchanging the Grant's
	// Refresh token until unlock is called.
	Lock(ctx context.Context) (unlock func(), err error)
	RefreshToken() (string, error)
	// SaveRefreshToken must persist durably before it returns: presenting
	// the superseded token later revokes the Grant.
	SaveRefreshToken(token string) error
}

// PartnerAppID is the Partner app whose Grant the client uses, or "" for an
// API client.
func (c *Client) PartnerAppID() string {
	if c.config.Grant == nil {
		return ""
	}
	return c.config.ClientID
}

// CodeExchange redeems the code a consent redirect carried.
type CodeExchange struct {
	Code string
	// RedirectURI is the one the consent URL carried, byte for byte.
	RedirectURI  string
	CodeVerifier string
}

// GrantExchangeError is a failed `authorization_code` or `refresh_token`
// exchange, with what the person must do about it, since tmh never retries
// one (ADR-0004).
type GrantExchangeError struct {
	Err      error
	Guidance string
}

// Error is the failure followed by what to do about it.
func (e *GrantExchangeError) Error() string {
	return strings.TrimSuffix(e.Err.Error(), ".") + ". " + e.Guidance
}

// Unwrap returns the failed exchange's error.
func (e *GrantExchangeError) Unwrap() error { return e.Err }

const (
	codeConnectionGuidance = "The code exchange was not retried: the code may have been consumed, and a code works only once. " +
		"Run `tmh login --partner-app` again"
	refreshConnectionGuidance = "The Refresh token exchange was not retried, because replaying a Refresh token the API may already have rotated revokes the Grant. " +
		"If the next command fails with invalid_grant, run `tmh login --partner-app` again"
	invalidGrantGuidance = "The Grant was revoked, or its Refresh token was reused (presenting a superseded Refresh token revokes the Grant). " +
		"Run `tmh login --partner-app` again"
)

// ExchangeCode redeems a consent code for the Grant's first tokens, exactly
// once. save receives the Refresh token before the Access token is used.
func (c *Client) ExchangeCode(ctx context.Context, exchange CodeExchange, save func(refreshToken string) error) error {
	tokens, err := c.exchangeGrant(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {exchange.Code},
		"redirect_uri":  {exchange.RedirectURI},
		"code_verifier": {exchange.CodeVerifier},
	}, codeConnectionGuidance)
	if err != nil {
		return err
	}
	if err := save(tokens.refreshToken); err != nil {
		return fmt.Errorf("could not store the Grant's Refresh token, so tmh is not logged in: %w", err)
	}
	c.useToken(tokens.accessToken, tokens.requestedAt, tokens.expiresIn)
	return nil
}

// refresh exchanges the Grant's Refresh token once, under the cross-process
// lock, and stores the rotated one before its Access token is used.
func (c *Client) refresh(ctx context.Context) error {
	unlock, err := c.config.Grant.Lock(ctx)
	if err != nil {
		return fmt.Errorf("waiting for another tmh process to finish refreshing the Grant: %w", err)
	}
	defer unlock()
	// Read only now: another process may have rotated the token while this
	// one waited for the lock.
	refreshToken, err := c.config.Grant.RefreshToken()
	if err != nil {
		return err
	}
	tokens, err := c.exchangeGrant(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}, refreshConnectionGuidance)
	if err != nil {
		return err
	}
	if err := c.config.Grant.SaveRefreshToken(tokens.refreshToken); err != nil {
		return fmt.Errorf("could not store the rotated Refresh token, so its Access token was not used: %w. "+
			"The stored Refresh token is superseded, and presenting it would revoke the Grant; run `tmh login --partner-app` again", err)
	}
	c.useToken(tokens.accessToken, tokens.requestedAt, tokens.expiresIn)
	return nil
}

type grantTokens struct {
	accessToken  string
	refreshToken string
	expiresIn    float64
	requestedAt  time.Time
}

// exchangeGrant posts form to the token endpoint once and never retries:
// replaying a spent code or a rotated Refresh token revokes the Grant.
func (c *Client) exchangeGrant(ctx context.Context, form url.Values, connectionGuidance string) (grantTokens, error) {
	requestedAt := c.config.Clock.Now()
	status, header, body, err := c.postToken(ctx, form)
	if err != nil {
		return grantTokens{}, &GrantExchangeError{Err: err, Guidance: connectionGuidance}
	}
	token, failure := parseTokenResponse(status, header, body)
	if failure != nil {
		if failure.OAuth && failure.Code == "invalid_grant" {
			return grantTokens{}, &GrantExchangeError{Err: failure, Guidance: invalidGrantGuidance}
		}
		return grantTokens{}, failure
	}
	if token.RefreshToken == "" {
		// The message leaves the body out: it holds a live Access token.
		return grantTokens{}, &Error{Status: status, RequestID: header.Get("X-Request-Id"), Header: header,
			UnexpectedBody: true, message: "the token endpoint answered without a refresh_token"}
	}
	return grantTokens{
		accessToken:  token.AccessToken,
		refreshToken: token.RefreshToken,
		expiresIn:    *token.ExpiresIn,
		requestedAt:  requestedAt,
	}, nil
}
