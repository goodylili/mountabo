package github

import (
	"context"
	"fmt"

	"github.com/goodylili/mountabo/internal/usecase"
	gogithub "github.com/google/go-github/v88/github"
)

var (
	_ usecase.RepoBranchLister = (*Client)(nil)
	_ usecase.RefResolver      = (*Client)(nil)
)

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

// HeadSHA returns the head commit SHA of owner/repo's branch, so a deploy can
// record the exact commit it releases. The single-branch GET resolves the tip
// without paging the whole branch list.
func (c *Client) HeadSHA(ctx context.Context, t usecase.Token, owner, repo, branch string) (string, error) {
	api, err := gogithub.NewClient(gogithub.WithAuthToken(t.AccessToken))
	if err != nil {
		return "", fmt.Errorf("build github client: %w", err)
	}

	b, _, err := api.Repositories.GetBranch(ctx, owner, repo, branch, 0)
	if err != nil {
		return "", fmt.Errorf("get branch %s for %s/%s: %w", branch, owner, repo, err)
	}
	return b.GetCommit().GetSHA(), nil
}
