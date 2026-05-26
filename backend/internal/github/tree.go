package github

import (
	"context"
	"fmt"

	"github.com/goodylili/mountabo/internal/usecase"
	gogithub "github.com/google/go-github/v88/github"
)

var _ usecase.RepoTreeLister = (*Client)(nil)

// Tree returns every path in owner/repo at ref. GitHub's git/trees endpoint
// accepts a branch name as the tree-ish and, with recursive set, returns the
// whole tree in a single call (GitHub truncates only very large trees, which
// the picker tolerates by simply listing fewer entries).
func (c *Client) Tree(ctx context.Context, t usecase.Token, owner, repo, ref string) ([]usecase.TreeEntry, error) {
	api, err := gogithub.NewClient(gogithub.WithAuthToken(t.AccessToken))
	if err != nil {
		return nil, fmt.Errorf("build github client: %w", err)
	}

	tree, _, err := api.Git.GetTree(ctx, owner, repo, ref, true)
	if err != nil {
		return nil, fmt.Errorf("get tree for %s/%s@%s: %w", owner, repo, ref, err)
	}

	entries := make([]usecase.TreeEntry, 0, len(tree.Entries))
	for _, e := range tree.Entries {
		entries = append(entries, usecase.TreeEntry{Path: e.GetPath(), Dir: e.GetType() == "tree"})
	}
	return entries, nil
}
