// Package github talks to GitHub on the user's behalf: it exchanges OAuth
// authorization codes for tokens and reads the account a token belongs to.
// These concrete types satisfy the ports declared in internal/usecase.
package github

import (
	"context"
	"fmt"

	"github.com/goodylili/mountabo/internal/usecase"
	"golang.org/x/oauth2"
	githuboauth "golang.org/x/oauth2/github"
)

// OAuth performs the GitHub App user-to-server web flow's token exchange. The
// client secret stays on this machine: it is only ever sent directly to
// GitHub's token endpoint over TLS and is never logged or returned to callers.
type OAuth struct {
	config *oauth2.Config
}

var (
	_ usecase.CodeExchanger  = (*OAuth)(nil)
	_ usecase.TokenRefresher = (*OAuth)(nil)
)

// NewOAuth builds an exchanger for the given GitHub App OAuth credentials. The
// redirect URI is supplied per-exchange (it depends on the request origin), so
// it is not fixed on the config here.
func NewOAuth(clientID, clientSecret string) *OAuth {
	return &OAuth{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint:     githuboauth.Endpoint,
		},
	}
}

// Exchange trades an authorization code for a user-to-server token. redirectURI
// must match the value used when the code was issued, or GitHub rejects the
// exchange.
func (o *OAuth) Exchange(ctx context.Context, code, redirectURI string) (usecase.Token, error) {
	if o.config.ClientID == "" || o.config.ClientSecret == "" {
		return usecase.Token{}, fmt.Errorf("github oauth not configured: set GITHUB_CLIENT_ID and GITHUB_CLIENT_SECRET")
	}

	tok, err := o.config.Exchange(ctx, code, oauth2.SetAuthURLParam("redirect_uri", redirectURI))
	if err != nil {
		return usecase.Token{}, fmt.Errorf("oauth token exchange: %w", err)
	}

	return usecase.Token{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		Expiry:       tok.Expiry,
	}, nil
}

// Refresh exchanges an expiring token's refresh token for a new token. GitHub
// rotates the refresh token on every refresh, so the returned token's refresh
// token must be persisted in place of the old one. The client credentials go
// to GitHub's token endpoint over TLS, never logged or returned.
func (o *OAuth) Refresh(ctx context.Context, t usecase.Token) (usecase.Token, error) {
	if t.RefreshToken == "" {
		return usecase.Token{}, fmt.Errorf("no refresh token to refresh with")
	}
	if o.config.ClientID == "" || o.config.ClientSecret == "" {
		return usecase.Token{}, fmt.Errorf("github oauth not configured: set GITHUB_CLIENT_ID and GITHUB_CLIENT_SECRET")
	}

	// A TokenSource seeded with the expired token refreshes on the first call to
	// Token() because the seed is no longer Valid().
	src := o.config.TokenSource(ctx, &oauth2.Token{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		Expiry:       t.Expiry,
	})
	tok, err := src.Token()
	if err != nil {
		return usecase.Token{}, fmt.Errorf("refresh oauth token: %w", err)
	}

	return usecase.Token{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		Expiry:       tok.Expiry,
	}, nil
}
