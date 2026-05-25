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

var _ usecase.CodeExchanger = (*OAuth)(nil)

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
