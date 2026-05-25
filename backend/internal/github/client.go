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

var _ usecase.AccountFetcher = (*Client)(nil)

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
