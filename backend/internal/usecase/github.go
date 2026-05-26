// Package usecase holds mountabo's application logic and the ports it depends
// on. It owns the interfaces it consumes; adapters in internal/github and
// internal/keychain satisfy them. No HTTP, OAuth, or GitHub library types leak
// across this boundary, only the domain types defined here.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// repoCacheTTL bounds how stale the cached repository list may be. Listing all
// of an account's repos is the slowest call in the app (full pagination against
// GitHub), so a short cache keeps repeat page loads instant while still picking
// up new repos within a minute.
const repoCacheTTL = 60 * time.Second

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

// Repo is a GitHub repository the connected account can deploy from.
type Repo struct {
	Owner         string
	Name          string
	FullName      string
	Private       bool
	DefaultBranch string
	Language      string
	PushedAt      time.Time
	// HasDocker is true when the repo's root holds a Dockerfile or a Compose
	// file, so the UI can flag repos that are ready to containerize.
	HasDocker bool
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

// RepoLister lists every repository a token can access, public and private,
// owned, collaborator, and organization.
type RepoLister interface {
	List(ctx context.Context, t Token) ([]Repo, error)
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
	repos     RepoLister
	tokens    TokenStore

	mu          sync.Mutex // guards the repo cache below
	repoCache   []Repo
	repoCacheAt time.Time
}

// NewGitHubConnector wires the connector to its ports.
func NewGitHubConnector(exchanger CodeExchanger, accounts AccountFetcher, repos RepoLister, tokens TokenStore) *GitHubConnector {
	return &GitHubConnector{exchanger: exchanger, accounts: accounts, repos: repos, tokens: tokens}
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
	c.invalidateRepoCache() // a fresh connection may be a different account
	return account, nil
}

// invalidateRepoCache drops any cached repository list so the next read fetches
// fresh. Called whenever the connected identity changes.
func (c *GitHubConnector) invalidateRepoCache() {
	c.mu.Lock()
	c.repoCache, c.repoCacheAt = nil, time.Time{}
	c.mu.Unlock()
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

// Repositories lists the connected account's repositories (public and private)
// using the stored token. It returns ErrNotConnected when no token is stored.
func (c *GitHubConnector) Repositories(ctx context.Context) ([]Repo, error) {
	c.mu.Lock()
	if c.repoCache != nil && time.Since(c.repoCacheAt) < repoCacheTTL {
		cached := c.repoCache
		c.mu.Unlock()
		return cached, nil
	}
	c.mu.Unlock()

	token, err := c.tokens.Load()
	if err != nil {
		return nil, fmt.Errorf("load token: %w", err)
	}

	repos, err := c.repos.List(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("list repositories: %w", err)
	}

	c.mu.Lock()
	c.repoCache, c.repoCacheAt = repos, time.Now()
	c.mu.Unlock()
	return repos, nil
}

// Disconnect removes the stored token from the keychain. It is idempotent: a
// connector with no stored token disconnects without error.
func (c *GitHubConnector) Disconnect() error {
	if err := c.tokens.Delete(); err != nil {
		return fmt.Errorf("delete token: %w", err)
	}
	c.invalidateRepoCache()
	return nil
}
