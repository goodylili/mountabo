package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/goodylili/mountabo/internal/usecase"
	gogithub "github.com/google/go-github/v88/github"
)

var _ usecase.WorkflowRunLister = (*Client)(nil)

// ListWorkflowRuns returns the most recent runs of workflowFile on branch,
// newest first. workflowFile is the file name under .github/workflows (e.g.
// "mountabo-deploy-main.yml").
func (c *Client) ListWorkflowRuns(ctx context.Context, t usecase.Token, owner, repo, workflowFile, branch string, limit int) ([]usecase.WorkflowRun, error) {
	api, err := gogithub.NewClient(gogithub.WithAuthToken(t.AccessToken))
	if err != nil {
		return nil, fmt.Errorf("build github client: %w", err)
	}

	opts := &gogithub.ListWorkflowRunsOptions{
		Branch:      branch,
		ListOptions: gogithub.ListOptions{PerPage: limit},
	}
	runs, _, err := api.Actions.ListWorkflowRunsByFileName(ctx, owner, repo, workflowFile, opts)
	if err != nil {
		return nil, fmt.Errorf("list runs for %s/%s %s: %w", owner, repo, workflowFile, err)
	}

	out := make([]usecase.WorkflowRun, 0, len(runs.WorkflowRuns))
	for _, r := range runs.WorkflowRuns {
		title := r.GetDisplayTitle()
		if title == "" {
			title = r.GetHeadCommit().GetMessage()
		}
		out = append(out, usecase.WorkflowRun{
			SHA:        r.GetHeadSHA(),
			Title:      firstLine(title),
			Status:     r.GetStatus(),
			Conclusion: r.GetConclusion(),
			CreatedAt:  r.GetCreatedAt().Time,
			UpdatedAt:  r.GetUpdatedAt().Time,
		})
	}
	return out, nil
}

// firstLine keeps a commit/run title to its first line for a tidy one-line row.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
