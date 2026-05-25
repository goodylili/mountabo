package github

import (
	"context"
	"fmt"

	"github.com/goodylili/mountabo/internal/usecase"
	gogithub "github.com/google/go-github/v88/github"
)

// Client reads from the GitHub API using a user-to-server token. It is stateless
// with respect to credentials: each call builds an authenticated client from the
// token it is given, so one Client can serve any connected account.
type Client struct{}

var (
	_ usecase.AccountFetcher = (*Client)(nil)
	_ usecase.RepoLister     = (*Client)(nil)
)

// NewClient returns a GitHub API client.
func NewClient() *Client {
	return &Client{}
}

// Account returns the GitHub account the token authenticates as. The empty
// username asks the API for the authenticated user behind the token.
func (c *Client) Account(ctx context.Context, t usecase.Token) (usecase.Account, error) {
	api, err := gogithub.NewClient(gogithub.WithAuthToken(t.AccessToken))
	if err != nil {
		return usecase.Account{}, fmt.Errorf("build github client: %w", err)
	}

	user, _, err := api.Users.Get(ctx, "")
	if err != nil {
		return usecase.Account{}, fmt.Errorf("get authenticated user: %w", err)
	}
	return usecase.Account{Login: user.GetLogin()}, nil
}

// List returns every repository the token can access, paging through all
// results. Visibility "all" includes private repos; the default affiliation
// covers owned, collaborator, and organization repositories.
func (c *Client) List(ctx context.Context, t usecase.Token) ([]usecase.Repo, error) {
	api, err := gogithub.NewClient(gogithub.WithAuthToken(t.AccessToken))
	if err != nil {
		return nil, fmt.Errorf("build github client: %w", err)
	}

	opt := &gogithub.RepositoryListByAuthenticatedUserOptions{
		Visibility:  "all",
		Affiliation: "owner,collaborator,organization_member",
		Sort:        "pushed",
		ListOptions: gogithub.ListOptions{PerPage: 100},
	}

	var repos []usecase.Repo
	for {
		page, resp, err := api.Repositories.ListByAuthenticatedUser(ctx, opt)
		if err != nil {
			return nil, fmt.Errorf("list repositories: %w", err)
		}
		for _, r := range page {
			repos = append(repos, usecase.Repo{
				Owner:         r.GetOwner().GetLogin(),
				Name:          r.GetName(),
				FullName:      r.GetFullName(),
				Private:       r.GetPrivate(),
				DefaultBranch: r.GetDefaultBranch(),
				Language:      r.GetLanguage(),
				PushedAt:      r.GetPushedAt().Time,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return repos, nil
}
