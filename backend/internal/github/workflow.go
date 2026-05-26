package github

import (
	"context"
	"fmt"

	"github.com/goodylili/mountabo/internal/usecase"
	gogithub "github.com/google/go-github/v88/github"
)

var _ usecase.RepoWriter = (*Client)(nil)

// PutFile creates path on owner/repo at branch, or updates it in place when it
// already exists. The contents API needs the current blob SHA to update, so it
// reads the existing file first; a missing file (or unreadable ref) is treated
// as a create. content is committed verbatim with the given message.
func (c *Client) PutFile(ctx context.Context, t usecase.Token, owner, repo, branch, path, content, message string) error {
	api, err := gogithub.NewClient(gogithub.WithAuthToken(t.AccessToken))
	if err != nil {
		return fmt.Errorf("build github client: %w", err)
	}

	var sha *string
	if file, _, _, gerr := api.Repositories.GetContents(ctx, owner, repo, path, &gogithub.RepositoryContentGetOptions{Ref: branch}); gerr == nil && file != nil {
		sha = file.SHA
	}

	opts := &gogithub.RepositoryContentFileOptions{
		Message: gogithub.Ptr(message),
		Content: []byte(content),
		Branch:  gogithub.Ptr(branch),
		SHA:     sha,
	}

	if sha == nil {
		_, _, err = api.Repositories.CreateFile(ctx, owner, repo, path, opts)
	} else {
		_, _, err = api.Repositories.UpdateFile(ctx, owner, repo, path, opts)
	}
	if err != nil {
		return fmt.Errorf("write %s to %s/%s: %w", path, owner, repo, err)
	}
	return nil
}
