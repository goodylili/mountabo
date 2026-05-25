// Package usecase holds mountabo's application logic and the ports it depends
// on. It owns the interfaces it consumes; adapters in internal/github and
// internal/keychain satisfy them. No HTTP, OAuth, or GitHub library types leak
// across this boundary — only the domain types defined here.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrNotConnected is returned when no GitHub token is stored yet.
var ErrNotConnected = errors.New("github not connected")

// Token is the credential mountabo holds for a connected GitHub account. For a
// GitHub App user-to-server authorization it may carry a refresh token and an
// expiry; both are empty/zero when the App issues non-expiring tokens.
type Token struct {
	AccessToken  string
	RefreshToken string
	Expiry       time.Time
}

// Account identifies the GitHub user a token authenticates as.
type Account struct {
	Login string
}

// CodeExchanger turns a completed OAuth web-flow authorization code into a
// token. redirectURI must match the one used to obtain the code.
type CodeExchanger interface {
	Exchange(ctx context.Context, code, redirectURI string) (Token, error)
}

// AccountFetcher reports which GitHub account a token authenticates as.
type AccountFetcher interface {
	Account(ctx context.Context, t Token) (Account, error)
}

// TokenStore persists the connection token in the OS keychain. Load returns
// ErrNotConnected when nothing is stored; Delete is idempotent.
type TokenStore interface {
	Save(t Token) error
	Load() (Token, error)
	Delete() error
}

// GitHubConnector runs the GitHub connection flow: exchange an OAuth code for a
// token, confirm the account it belongs to, and store the token so later
// operations (deploy keys, secrets, workflow files) can reuse it.
type GitHubConnector struct {
	exchanger CodeExchanger
	accounts  AccountFetcher
	tokens    TokenStore
}

// NewGitHubConnector wires the connector to its ports.
func NewGitHubConnector(exchanger CodeExchanger, accounts AccountFetcher, tokens TokenStore) *GitHubConnector {
	return &GitHubConnector{exchanger: exchanger, accounts: accounts, tokens: tokens}
}

// Connect exchanges code for a token, verifies it by reading the account it
// belongs to, then stores it. The token is only persisted after it is proven
// usable, so a failed exchange or a bad token never leaves a dead credential in
// the keychain. The token itself never crosses back to the caller.
func (c *GitHubConnector) Connect(ctx context.Context, code, redirectURI string) (Account, error) {
	token, err := c.exchanger.Exchange(ctx, code, redirectURI)
	if err != nil {
		return Account{}, fmt.Errorf("exchange authorization code: %w", err)
	}

	account, err := c.accounts.Account(ctx, token)
	if err != nil {
		return Account{}, fmt.Errorf("read connected account: %w", err)
	}

	if err := c.tokens.Save(token); err != nil {
		return Account{}, fmt.Errorf("store token: %w", err)
	}
	return account, nil
}

// Status reports the connected account using the stored token, refreshing the
// caller's view of who mountabo is acting as. It returns ErrNotConnected when
// no token is stored.
func (c *GitHubConnector) Status(ctx context.Context) (Account, error) {
	token, err := c.tokens.Load()
	if err != nil {
		return Account{}, fmt.Errorf("load token: %w", err)
	}

	account, err := c.accounts.Account(ctx, token)
	if err != nil {
		return Account{}, fmt.Errorf("read connected account: %w", err)
	}
	return account, nil
}

// Disconnect removes the stored token from the keychain. It is idempotent: a
// connector with no stored token disconnects without error.
func (c *GitHubConnector) Disconnect() error {
	if err := c.tokens.Delete(); err != nil {
		return fmt.Errorf("delete token: %w", err)
	}
	return nil
}
