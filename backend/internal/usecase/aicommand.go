package usecase

import (
	"context"
	"fmt"
	"strings"
)

// CommandSuggestion is the model's answer to a plain-English request: a shell
// command and a short explanation of what it does. Command may be empty when the
// request is impossible or unclear, in which case Explanation says why.
type CommandSuggestion struct {
	Command     string `json:"command"`
	Explanation string `json:"explanation"`
}

// CommandSuggester turns a plain-English request into a suggested shell command.
// Configured reports whether the underlying provider has credentials, so the
// service can give a clear "not configured" answer instead of attempting a call
// that would fail.
type CommandSuggester interface {
	Configured() bool
	Suggest(ctx context.Context, prompt, userContext string) (CommandSuggestion, error)
}

// AICommandResult wraps a suggestion with whether AI is available at all, so the
// handler can return a clean 200 in the "not configured" case (the UI shows a
// hint to set ANTHROPIC_API_KEY) rather than a 500.
type AICommandResult struct {
	Configured  bool   `json:"configured"`
	Command     string `json:"command"`
	Explanation string `json:"explanation"`
}

// AICommandService suggests a shell command for a plain-English request. It is
// strictly advisory: the suggestion is shown to the operator, who reviews and
// runs it themselves. This service never touches a server and never executes a
// command.
type AICommandService struct {
	suggester CommandSuggester
}

// NewAICommandService wires the service to its suggester port.
func NewAICommandService(suggester CommandSuggester) *AICommandService {
	return &AICommandService{suggester: suggester}
}

// notConfiguredHint is returned (with Configured=false) when no API key is set,
// so the UI can tell the operator exactly what to do.
const notConfiguredHint = "AI command suggestions are not configured. Set ANTHROPIC_API_KEY in the environment (or .env) and restart mountabo to enable them."

// Suggest returns a command suggestion for prompt. When the suggester has no
// credentials it returns a structured not-configured result (not an error), so
// the absence of a key is a normal, displayable state rather than a failure.
func (s *AICommandService) Suggest(ctx context.Context, prompt, userContext string) (AICommandResult, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return AICommandResult{}, fmt.Errorf("prompt is required")
	}
	if !s.suggester.Configured() {
		return AICommandResult{Configured: false, Explanation: notConfiguredHint}, nil
	}

	suggestion, err := s.suggester.Suggest(ctx, prompt, userContext)
	if err != nil {
		return AICommandResult{}, fmt.Errorf("suggest command: %w", err)
	}
	return AICommandResult{
		Configured:  true,
		Command:     suggestion.Command,
		Explanation: suggestion.Explanation,
	}, nil
}
