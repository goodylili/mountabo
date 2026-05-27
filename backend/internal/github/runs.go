package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/goodylili/mountabo/internal/usecase"
	gogithub "github.com/google/go-github/v88/github"
)

var (
	_ usecase.WorkflowRunLister = (*Client)(nil)
	_ usecase.WorkflowJobLister = (*Client)(nil)
)

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
			HTMLURL:    r.GetHTMLURL(),
		})
	}
	return out, nil
}

// LatestRun returns the most recent run of workflowFile on branch with its jobs
// and their steps, so the UI can show each Actions step's live status. A
// workflow that has no run yet (just configured, never pushed) yields a zero
// RunSteps and no error.
func (c *Client) LatestRun(ctx context.Context, t usecase.Token, owner, repo, workflowFile, branch string) (usecase.RunSteps, error) {
	api, err := gogithub.NewClient(gogithub.WithAuthToken(t.AccessToken))
	if err != nil {
		return usecase.RunSteps{}, fmt.Errorf("build github client: %w", err)
	}

	runOpts := &gogithub.ListWorkflowRunsOptions{
		Branch:      branch,
		ListOptions: gogithub.ListOptions{PerPage: 1},
	}
	runs, _, err := api.Actions.ListWorkflowRunsByFileName(ctx, owner, repo, workflowFile, runOpts)
	if err != nil {
		return usecase.RunSteps{}, fmt.Errorf("list runs for %s/%s %s: %w", owner, repo, workflowFile, err)
	}
	if len(runs.WorkflowRuns) == 0 {
		return usecase.RunSteps{}, nil // configured but never run yet
	}

	run := runs.WorkflowRuns[0]
	steps := usecase.RunSteps{
		RunURL:     run.GetHTMLURL(),
		Status:     run.GetStatus(),
		Conclusion: run.GetConclusion(),
	}

	jobs, _, err := api.Actions.ListWorkflowJobs(ctx, owner, repo, run.GetID(), &gogithub.ListWorkflowJobsOptions{})
	if err != nil {
		return usecase.RunSteps{}, fmt.Errorf("list jobs for run %d: %w", run.GetID(), err)
	}
	steps.Jobs = make([]usecase.RunJob, 0, len(jobs.Jobs))
	for _, j := range jobs.Jobs {
		job := usecase.RunJob{
			Name:       j.GetName(),
			Status:     j.GetStatus(),
			Conclusion: j.GetConclusion(),
			HTMLURL:    j.GetHTMLURL(),
			Steps:      make([]usecase.RunStep, 0, len(j.Steps)),
		}
		for _, st := range j.Steps {
			job.Steps = append(job.Steps, usecase.RunStep{
				Name:       st.GetName(),
				Status:     st.GetStatus(),
				Conclusion: st.GetConclusion(),
				Number:     int(st.GetNumber()),
			})
		}
		steps.Jobs = append(steps.Jobs, job)
	}
	return steps, nil
}

// firstLine keeps a commit/run title to its first line for a tidy one-line row.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
