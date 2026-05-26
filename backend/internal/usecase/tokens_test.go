package usecase

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeRefresher struct {
	out    Token
	err    error
	called bool
}

func (f *fakeRefresher) Refresh(context.Context, Token) (Token, error) {
	f.called = true
	return f.out, f.err
}

func TestTokenManager_NonExpiringReturnedUnchanged(t *testing.T) {
	store := &fakeStore{loadTok: Token{AccessToken: "tok"}} // zero Expiry, no refresh token
	ref := &fakeRefresher{}
	m := NewTokenManager(store, ref)

	got, err := m.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.AccessToken != "tok" {
		t.Errorf("access token = %q, want tok", got.AccessToken)
	}
	if ref.called {
		t.Error("refresh should not run for a non-expiring token")
	}
	if store.saved != nil {
		t.Error("nothing should be persisted when no refresh happens")
	}
}

func TestTokenManager_ExpiredTokenRefreshedAndPersisted(t *testing.T) {
	store := &fakeStore{loadTok: Token{
		AccessToken:  "old",
		RefreshToken: "refresh",
		Expiry:       time.Now().Add(-time.Hour),
	}}
	ref := &fakeRefresher{out: Token{AccessToken: "new", RefreshToken: "refresh2", Expiry: time.Now().Add(time.Hour)}}
	m := NewTokenManager(store, ref)

	got, err := m.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !ref.called {
		t.Fatal("expected a refresh for an expired token")
	}
	if got.AccessToken != "new" {
		t.Errorf("access token = %q, want new", got.AccessToken)
	}
	if store.saved == nil || store.saved.AccessToken != "new" || store.saved.RefreshToken != "refresh2" {
		t.Errorf("refreshed token not persisted, got %+v", store.saved)
	}
}

func TestTokenManager_ExpiredButNoRefreshTokenReturnedAsIs(t *testing.T) {
	store := &fakeStore{loadTok: Token{AccessToken: "old", Expiry: time.Now().Add(-time.Hour)}}
	ref := &fakeRefresher{}
	m := NewTokenManager(store, ref)

	got, err := m.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if ref.called {
		t.Error("cannot refresh without a refresh token")
	}
	if got.AccessToken != "old" {
		t.Errorf("access token = %q, want old", got.AccessToken)
	}
}

func TestTokenManager_RefreshErrorPropagates(t *testing.T) {
	store := &fakeStore{loadTok: Token{AccessToken: "old", RefreshToken: "r", Expiry: time.Now().Add(-time.Hour)}}
	ref := &fakeRefresher{err: errors.New("github said no")}
	m := NewTokenManager(store, ref)

	if _, err := m.Load(); err == nil {
		t.Fatal("expected the refresh error to propagate")
	}
}

func TestTokenManager_NotConnectedPropagates(t *testing.T) {
	store := &fakeStore{loadErr: ErrNotConnected}
	m := NewTokenManager(store, &fakeRefresher{})

	if _, err := m.Load(); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("want ErrNotConnected, got %v", err)
	}
}
