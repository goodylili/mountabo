package github

import (
	"context"
	"fmt"

	"github.com/goodylili/mountabo/internal/usecase"
	gogithub "github.com/google/go-github/v88/github"
)

// AddDeployKey registers publicKey as a deploy key on owner/repo and returns the
// created key's id. readOnly should be true for mountabo's key, the server only
// needs to pull, never push, which keeps the key's access scoped to one repo.
// Adding the same key again returns an error from GitHub ("key is already in
// use"); callers that may re-run should treat that as benign.
func (c *Client) AddDeployKey(ctx context.Context, t usecase.Token, owner, repo, title, publicKey string, readOnly bool) (int64, error) {
	api, err := gogithub.NewClient(gogithub.WithAuthToken(t.AccessToken))
	if err != nil {
		return 0, fmt.Errorf("build github client: %w", err)
	}

	key, _, err := api.Repositories.CreateKey(ctx, owner, repo, &gogithub.Key{
		Title:    gogithub.Ptr(title),
		Key:      gogithub.Ptr(publicKey),
		ReadOnly: gogithub.Ptr(readOnly),
	})
	if err != nil {
		return 0, fmt.Errorf("create deploy key on %s/%s: %w", owner, repo, err)
	}
	return key.GetID(), nil
}

// RemoveDeployKey deletes a deploy key from owner/repo by id (for teardown).
func (c *Client) RemoveDeployKey(ctx context.Context, t usecase.Token, owner, repo string, keyID int64) error {
	api, err := gogithub.NewClient(gogithub.WithAuthToken(t.AccessToken))
	if err != nil {
		return fmt.Errorf("build github client: %w", err)
	}
	if _, err := api.Repositories.DeleteKey(ctx, owner, repo, keyID); err != nil {
		return fmt.Errorf("delete deploy key %d on %s/%s: %w", keyID, owner, repo, err)
	}
	return nil
}
