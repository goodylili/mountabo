package usecase

import (
	"context"
	"fmt"
)

// RunStep is one step of a GitHub Actions job in the latest deploy run.
type RunStep struct {
	Name       string `json:"name"`
	Status     string `json:"status"`     // queued | in_progress | completed
	Conclusion string `json:"conclusion"` // success | failure | ... (set once completed)
	Number     int    `json:"number"`
}

// RunJob is one job of the latest deploy run, with its ordered steps. ID
// identifies the job for fetching its full log.
type RunJob struct {
	ID         int64     `json:"jobId"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion"`
	HTMLURL    string    `json:"htmlUrl"`
	Steps      []RunStep `json:"steps"`
}

// RunSteps is the latest deploy run's live progress: its overall status and the
// jobs (with their steps) GitHub Actions is running. Empty (RunURL "" and no
// jobs) when the workflow has no run yet, which is a normal answer, not an error.
type RunSteps struct {
	RunURL     string   `json:"runUrl"`
	Status     string   `json:"status"`
	Conclusion string   `json:"conclusion"`
	Jobs       []RunJob `json:"jobs"`
}

// WorkflowJobLister reads the jobs and steps of a workflow run, and a single
// job's full log.
//
// LatestRun finds the most recent run of workflowFile on branch and returns
// that run with its jobs and their steps. A workflow with no run yet yields a
// zero RunSteps and a nil error. JobLog returns one job's plain-text log split
// into lines, so the UI can show what each step printed and why a step failed.
type WorkflowJobLister interface {
	LatestRun(ctx context.Context, t Token, owner, repo, workflowFile, branch string) (RunSteps, error)
	JobLog(ctx context.Context, t Token, owner, repo string, jobID int64) ([]string, error)
}

// RunStepsService reports the latest deploy run's job/step progress on the
// connected user's behalf, so the UI can show each Actions step's live status.
// It reads with the stored OAuth token; nothing else is needed.
type RunStepsService struct {
	tokens TokenStore
	jobs   WorkflowJobLister
}

// NewRunStepsService wires the service to the token store and a job lister.
func NewRunStepsService(tokens TokenStore, jobs WorkflowJobLister) *RunStepsService {
	return &RunStepsService{tokens: tokens, jobs: jobs}
}

// Steps returns the latest run of the deploy workflow for owner/repo on branch,
// with its jobs and steps. ref is the branch (defaulting to the repo's deploy
// branch convention is the caller's job; an empty ref reads the default
// branch's runs). It derives the workflow file from the branch, matching the
// name the deploy generator commits. It returns ErrNotConnected when no token is
// stored, and an empty RunSteps (no error) when there is no run yet.
func (s *RunStepsService) Steps(ctx context.Context, owner, repo, ref string) (RunSteps, error) {
	if owner == "" || repo == "" {
		return RunSteps{}, fmt.Errorf("owner and repo are required")
	}
	token, err := s.tokens.Load()
	if err != nil {
		return RunSteps{}, fmt.Errorf("load token: %w", err)
	}
	workflowFile := fmt.Sprintf("mountabo-deploy-%s.yml", ref)
	steps, err := s.jobs.LatestRun(ctx, token, owner, repo, workflowFile, ref)
	if err != nil {
		return RunSteps{}, fmt.Errorf("read run steps: %w", err)
	}
	return steps, nil
}

// JobLog returns one job's log lines for owner/repo, so the walkthrough can show
// what each step printed and what failed. It returns ErrNotConnected when no
// token is stored.
func (s *RunStepsService) JobLog(ctx context.Context, owner, repo string, jobID int64) ([]string, error) {
	if owner == "" || repo == "" || jobID == 0 {
		return nil, fmt.Errorf("owner, repo and jobId are required")
	}
	token, err := s.tokens.Load()
	if err != nil {
		return nil, fmt.Errorf("load token: %w", err)
	}
	lines, err := s.jobs.JobLog(ctx, token, owner, repo, jobID)
	if err != nil {
		return nil, fmt.Errorf("read job log: %w", err)
	}
	return lines, nil
}
