package github

import (
	"context"
	"fmt"

	"github.com/goodylili/mountabo/internal/usecase"
	gogithub "github.com/google/go-github/v88/github"
	"golang.org/x/sync/errgroup"
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

// List returns every repository the token can access. Visibility "all" includes
// private repos; the default affiliation covers owned, collaborator, and org
// repositories. It fetches page 1 to learn the page count, then fetches the rest
// concurrently (bounded) rather than walking NextPage one round-trip at a time —
// for accounts with hundreds of repos this turns ~7s of serial calls into ~1.
func (c *Client) List(ctx context.Context, t usecase.Token) ([]usecase.Repo, error) {
	api, err := gogithub.NewClient(gogithub.WithAuthToken(t.AccessToken))
	if err != nil {
		return nil, fmt.Errorf("build github client: %w", err)
	}

	opt := gogithub.RepositoryListByAuthenticatedUserOptions{
		Visibility:  "all",
		Affiliation: "owner,collaborator,organization_member",
		Sort:        "pushed",
		ListOptions: gogithub.ListOptions{PerPage: 100, Page: 1},
	}

	first, resp, err := api.Repositories.ListByAuthenticatedUser(ctx, &opt)
	if err != nil {
		return nil, fmt.Errorf("list repositories: %w", err)
	}

	// pages[i] holds page i+1; index 0 is the page we already have.
	pages := make([][]*gogithub.Repository, max(resp.LastPage, 1))
	pages[0] = first

	if resp.LastPage > 1 {
		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(6) // bound concurrent GitHub calls
		for p := 2; p <= resp.LastPage; p++ {
			pageOpt := opt // copy; each goroutine gets its own page number
			pageOpt.Page = p
			idx := p - 1 // distinct slice index per goroutine — no shared write
			g.Go(func() error {
				repos, _, err := api.Repositories.ListByAuthenticatedUser(gctx, &pageOpt)
				if err != nil {
					return fmt.Errorf("list repositories page %d: %w", pageOpt.Page, err)
				}
				pages[idx] = repos
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return nil, err
		}
	}

	var repos []usecase.Repo
	for _, page := range pages {
		for _, r := range page {
			repos = append(repos, toRepo(r))
		}
	}
	return repos, nil
}

func toRepo(r *gogithub.Repository) usecase.Repo {
	return usecase.Repo{
		Owner:         r.GetOwner().GetLogin(),
		Name:          r.GetName(),
		FullName:      r.GetFullName(),
		Private:       r.GetPrivate(),
		DefaultBranch: r.GetDefaultBranch(),
		Language:      r.GetLanguage(),
		PushedAt:      r.GetPushedAt().Time,
	}
}
