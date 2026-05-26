package usecase

import (
	"context"
	"fmt"
)

// TreeEntry is one path in a repository's file tree. Dir is true for a
// directory (so the UI can offer a directory picker), false for a file.
type TreeEntry struct {
	Path string `json:"path"`
	Dir  bool   `json:"dir"`
}

// RepoTreeLister lists every path in a repo at a ref in one read.
type RepoTreeLister interface {
	Tree(ctx context.Context, t Token, owner, repo, ref string) ([]TreeEntry, error)
}

// TreeService lists a repo's tree on the connected user's behalf, so the UI can
// offer a real directory/file picker instead of a free-text path. It reads with
// the stored OAuth token; nothing else is needed.
type TreeService struct {
	tokens TokenStore
	lister RepoTreeLister
}

// NewTreeService wires the service to the token store and a tree lister.
func NewTreeService(tokens TokenStore, lister RepoTreeLister) *TreeService {
	return &TreeService{tokens: tokens, lister: lister}
}

// Tree returns every path in owner/repo at ref. It returns ErrNotConnected when
// no token is stored.
func (s *TreeService) Tree(ctx context.Context, owner, repo, ref string) ([]TreeEntry, error) {
	if owner == "" || repo == "" || ref == "" {
		return nil, fmt.Errorf("owner, repo and ref are required")
	}
	token, err := s.tokens.Load()
	if err != nil {
		return nil, fmt.Errorf("load token: %w", err)
	}
	entries, err := s.lister.Tree(ctx, token, owner, repo, ref)
	if err != nil {
		return nil, fmt.Errorf("list tree: %w", err)
	}
	return entries, nil
}
