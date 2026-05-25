// Package keychain persists mountabo's secrets in the operating system's native
// credential store — macOS Keychain, Windows Credential Manager, or libsecret on
// Linux — so tokens are encrypted at rest by the OS and never written to
// mountabo's own files. The concrete Store satisfies the usecase.TokenStore port.
package keychain

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/goodylili/mountabo/internal/usecase"
	"github.com/zalando/go-keyring"
)

// service and tokenKey identify mountabo's GitHub token entry within the OS
// credential store.
const (
	service  = "mountabo"
	tokenKey = "github-token"
)

// Store reads and writes mountabo's GitHub token in the OS keychain.
type Store struct{}

var _ usecase.TokenStore = (*Store)(nil)

// NewStore returns a keychain-backed token store.
func NewStore() *Store {
	return &Store{}
}

// Save serialises the token and writes it to the keychain, replacing any
// existing entry.
func (s *Store) Save(t usecase.Token) error {
	blob, err := json.Marshal(t) //nolint:gosec // token is marshaled only to persist it in the OS keychain (encrypted at rest); never logged or written to disk
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}
	if err := keyring.Set(service, tokenKey, string(blob)); err != nil {
		return fmt.Errorf("write token to keychain: %w", err)
	}
	return nil
}

// Load reads the stored token, returning usecase.ErrNotConnected when none has
// been saved yet.
func (s *Store) Load() (usecase.Token, error) {
	blob, err := keyring.Get(service, tokenKey)
	if errors.Is(err, keyring.ErrNotFound) {
		return usecase.Token{}, usecase.ErrNotConnected
	}
	if err != nil {
		return usecase.Token{}, fmt.Errorf("read token from keychain: %w", err)
	}

	var t usecase.Token
	if err := json.Unmarshal([]byte(blob), &t); err != nil {
		return usecase.Token{}, fmt.Errorf("unmarshal token: %w", err)
	}
	return t, nil
}

// Delete removes the stored token. It is idempotent: deleting when nothing is
// stored is not an error.
func (s *Store) Delete() error {
	err := keyring.Delete(service, tokenKey)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete token from keychain: %w", err)
	}
	return nil
}
