package github

import (
	"context"
	"fmt"

	"github.com/goodylili/mountabo/internal/usecase"
	gogithub "github.com/google/go-github/v88/github"
)

var _ usecase.RepoBranchLister = (*Client)(nil)

// ListBranches pages through every branch on owner/repo and returns their
// names in GitHub's order (alphabetical). The page size matches the API's max
// so most repos are one request.
func (c *Client) ListBranches(ctx context.Context, t usecase.Token, owner, repo string) ([]string, error) {
	api, err := gogithub.NewClient(gogithub.WithAuthToken(t.AccessToken))
	if err != nil {
		return nil, fmt.Errorf("build github client: %w", err)
	}

	opts := &gogithub.BranchListOptions{
		ListOptions: gogithub.ListOptions{PerPage: 100},
	}
	out := []string{}
	for {
		branches, resp, err := api.Repositories.ListBranches(ctx, owner, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("list branches for %s/%s: %w", owner, repo, err)
		}
		for _, b := range branches {
			out = append(out, b.GetName())
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}
