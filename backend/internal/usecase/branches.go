package usecase

import (
	"context"
	"fmt"
)

// RepoBranchLister lists every branch on a repository at the moment of the
// call, ordered by GitHub's default (alphabetical). It is one read per repo and
// the result is small enough to ship straight to the UI.
type RepoBranchLister interface {
	ListBranches(ctx context.Context, t Token, owner, repo string) ([]string, error)
}

// BranchesService lists a repo's branches on the connected user's behalf, so
// the "add another environment" picker can present the real branch list
// instead of a free-text field. It reads with the stored OAuth token.
type BranchesService struct {
	tokens TokenStore
	lister RepoBranchLister
}

// NewBranchesService wires the service to the token store and a branch lister.
func NewBranchesService(tokens TokenStore, lister RepoBranchLister) *BranchesService {
	return &BranchesService{tokens: tokens, lister: lister}
}

// List returns every branch on owner/repo. It returns ErrNotConnected when no
// token is stored.
func (s *BranchesService) List(ctx context.Context, owner, repo string) ([]string, error) {
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner and repo are required")
	}
	token, err := s.tokens.Load()
	if err != nil {
		return nil, fmt.Errorf("load token: %w", err)
	}
	branches, err := s.lister.ListBranches(ctx, token, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("list branches: %w", err)
	}
	return branches, nil
}
