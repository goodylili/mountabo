package github

import (
	"context"
	"fmt"

	"github.com/goodylili/mountabo/internal/usecase"
	gogithub "github.com/google/go-github/v88/github"
)

var (
	_ usecase.RepoWriter      = (*Client)(nil)
	_ usecase.RepoFileDeleter = (*Client)(nil)
)

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

// DeleteFile removes path from owner/repo on branch via the contents API. The
// API needs the current blob SHA to delete, so it reads the file first; a file
// that is already gone (or an unreadable ref) is treated as a no-op success, so
// tearing down a deployment never fails on an already-removed workflow.
func (c *Client) DeleteFile(ctx context.Context, t usecase.Token, owner, repo, branch, path, message string) error {
	api, err := gogithub.NewClient(gogithub.WithAuthToken(t.AccessToken))
	if err != nil {
		return fmt.Errorf("build github client: %w", err)
	}

	file, _, _, gerr := api.Repositories.GetContents(ctx, owner, repo, path, &gogithub.RepositoryContentGetOptions{Ref: branch})
	if gerr != nil {
		// The file (or its ref) could not be read, almost always because it is
		// already gone: a teardown must not fail on an already-removed file, so
		// this is a clean no-op rather than an error.
		return nil //nolint:nilerr // missing file on teardown is success, not a failure
	}
	if file == nil || file.SHA == nil {
		// Path is a directory or carries no blob SHA: nothing single-file to delete.
		return nil
	}

	opts := &gogithub.RepositoryContentFileOptions{
		Message: gogithub.Ptr(message),
		Branch:  gogithub.Ptr(branch),
		SHA:     file.SHA,
	}
	if _, _, err := api.Repositories.DeleteFile(ctx, owner, repo, path, opts); err != nil {
		return fmt.Errorf("delete %s from %s/%s: %w", path, owner, repo, err)
	}
	return nil
}
