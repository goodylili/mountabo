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

// Store reads and writes mountabo's secrets in the OS keychain — the GitHub
// token and, generically, per-server secrets (root passwords, generated keys).
type Store struct{}

var (
	_ usecase.TokenStore  = (*Store)(nil)
	_ usecase.SecretVault = (*Store)(nil)
)

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

// SaveSecret stores an arbitrary secret under key, replacing any existing value.
func (s *Store) SaveSecret(key, value string) error {
	if err := keyring.Set(service, key, value); err != nil {
		return fmt.Errorf("write secret to keychain: %w", err)
	}
	return nil
}

// LoadSecret reads a secret by key, returning usecase.ErrNotConnected-free
// errors; a missing key yields an error so callers can react.
func (s *Store) LoadSecret(key string) (string, error) {
	v, err := keyring.Get(service, key)
	if err != nil {
		return "", fmt.Errorf("read secret from keychain: %w", err)
	}
	return v, nil
}

// DeleteSecret removes a secret by key. It is idempotent.
func (s *Store) DeleteSecret(key string) error {
	err := keyring.Delete(service, key)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete secret from keychain: %w", err)
	}
	return nil
}
