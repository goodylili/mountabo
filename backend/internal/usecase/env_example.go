package usecase

import (
	"context"
	"fmt"
)

// EnvExampleReader reads the variable names declared in a repo's example env
// file (.env.example or a common variant) at the given ref and directory. Only
// the keys are returned, never the example values. A repo with no example file
// yields an empty slice, not an error.
type EnvExampleReader interface {
	EnvExampleKeys(ctx context.Context, t Token, ref RepoRef) ([]string, error)
}

// EnvExampleService reads a repo's example env var names on the connected user's
// behalf, so the configure UI can pre-fill the env var rows for the operator to
// fill in. It reads with the stored OAuth token; nothing else is needed.
type EnvExampleService struct {
	tokens TokenStore
	reader EnvExampleReader
}

// NewEnvExampleService wires the service to the token store and an example
// reader.
func NewEnvExampleService(tokens TokenStore, reader EnvExampleReader) *EnvExampleService {
	return &EnvExampleService{tokens: tokens, reader: reader}
}

// Keys returns the variable names declared in owner/repo's example env file at
// ref (within the given sub-directory). It returns ErrNotConnected when no token
// is stored, and an empty slice (no error) when the repo has no example file.
func (s *EnvExampleService) Keys(ctx context.Context, ref RepoRef) ([]string, error) {
	if ref.Owner == "" || ref.Name == "" {
		return nil, fmt.Errorf("owner and repo are required")
	}
	token, err := s.tokens.Load()
	if err != nil {
		return nil, fmt.Errorf("load token: %w", err)
	}
	keys, err := s.reader.EnvExampleKeys(ctx, token, ref)
	if err != nil {
		return nil, fmt.Errorf("read env example: %w", err)
	}
	return keys, nil
}
