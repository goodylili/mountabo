package usecase

import (
	"context"
	"fmt"
	"time"
)

// expirySkew refreshes a token slightly before it actually expires, so a
// request that starts just before the deadline still goes out with a valid
// token. refreshTimeout bounds the refresh network call (Load has no caller
// context to thread through).
const (
	expirySkew     = 60 * time.Second
	refreshTimeout = 15 * time.Second
)

// TokenRefresher trades an expiring token (and its refresh token) for a fresh
// one. Implemented by the OAuth adapter, which holds the client credentials and
// talks to GitHub's token endpoint.
type TokenRefresher interface {
	Refresh(ctx context.Context, t Token) (Token, error)
}

// TokenManager keeps the connected account's OAuth token usable for every
// GitHub request. GitHub App user-to-server tokens expire (~8h); this wraps the
// keychain-backed store so a Load returns a currently-valid token, refreshing
// and persisting the rotated token when the stored one has expired. It
// satisfies TokenStore, so the connector and deploy service consume it exactly
// like a plain store, no extra wiring at the call sites.
type TokenManager struct {
	store     TokenStore
	refresher TokenRefresher
}

var _ TokenStore = (*TokenManager)(nil)

// NewTokenManager wraps a token store with automatic refresh.
func NewTokenManager(store TokenStore, refresher TokenRefresher) *TokenManager {
	return &TokenManager{store: store, refresher: refresher}
}

// Save persists a token (e.g. the one just obtained on connect, which carries
// the refresh token used later).
func (m *TokenManager) Save(t Token) error { return m.store.Save(t) }

// Delete forgets the stored token.
func (m *TokenManager) Delete() error { return m.store.Delete() }

// Load returns a valid token. A non-expiring token (zero expiry, or no refresh
// token to rotate with) is returned unchanged; an expired one is refreshed and
// the new token persisted before it is returned. ErrNotConnected from the
// underlying store propagates unchanged.
func (m *TokenManager) Load() (Token, error) {
	t, err := m.store.Load()
	if err != nil {
		return Token{}, err
	}
	if !needsRefresh(t) {
		return t, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
	defer cancel()

	refreshed, err := m.refresher.Refresh(ctx, t)
	if err != nil {
		return Token{}, fmt.Errorf("refresh github token: %w", err)
	}
	if err := m.store.Save(refreshed); err != nil {
		return Token{}, fmt.Errorf("persist refreshed github token: %w", err)
	}
	return refreshed, nil
}

// needsRefresh reports whether t has a refresh token and an expiry that has
// passed (within the skew). Tokens with no expiry never need refreshing.
func needsRefresh(t Token) bool {
	return t.RefreshToken != "" && !t.Expiry.IsZero() && time.Now().After(t.Expiry.Add(-expirySkew))
}
