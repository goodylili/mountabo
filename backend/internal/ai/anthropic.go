// Package ai talks to the Anthropic Messages API to turn a plain-English request
// into a suggested shell command. It is a thin adapter over the HTTP API (no SDK
// dependency): it sends a cached system prompt and the operator's request, and
// parses the model's JSON reply. It satisfies the CommandSuggester port declared
// in internal/usecase.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/goodylili/mountabo/internal/usecase"
)

const (
	// messagesURL is the Anthropic Messages endpoint. apiVersion pins the API
	// version header the endpoint requires.
	messagesURL = "https://api.anthropic.com/v1/messages"
	apiVersion  = "2023-06-01"

	// defaultModel is the model used for command suggestions. Sonnet is the
	// sensible default for this latency-sensitive, well-scoped task; the most
	// capable option is an Opus model, swap MOUNTABO_AI_MODEL to use it.
	defaultModel = "claude-sonnet-4-6"

	// maxTokens bounds the reply: a command plus a short explanation is small.
	maxTokens = 1024

	// requestTimeout bounds a single API call.
	requestTimeout = 30 * time.Second
)

// systemPrompt is the instruction block sent with every request. It is marked
// for prompt caching (see buildRequest): it is large and identical on every
// call, so caching it cuts input-token cost and latency after the first hit.
const systemPrompt = `You are a Linux server operations assistant embedded in mountabo, a tool that manages a user's own VPS over SSH. The user describes, in plain English, something they want to do on their server. You reply with the shell command (or a short pipeline) that accomplishes it, plus a one or two sentence explanation of what it does.

Rules:
- Target a modern Debian/Ubuntu server. The command runs as a non-root sudo-capable user named "mountabo"; prefix with "sudo" only when root is genuinely required.
- Prefer a single safe command. If several steps are unavoidable, join them clearly.
- Never include destructive commands (rm -rf /, mkfs, dd to a disk, fork bombs) unless the user explicitly and unambiguously asks; if they do, still return it but make the explanation warn plainly what it destroys.
- Do not invent flags or tools that do not exist. If the request is impossible or unclear, return an empty command and explain what is missing.
- The human reviews and runs the command themselves; you never execute anything.

Respond ONLY with a JSON object, no markdown, no prose around it, of the exact shape:
{"command": "<the shell command, or empty string if none>", "explanation": "<short plain explanation>"}`

// Client calls the Anthropic Messages API. The API key is held here, never
// logged, and read once at construction from the environment by the caller.
type Client struct {
	apiKey string
	model  string
	http   *http.Client
}

var _ usecase.CommandSuggester = (*Client)(nil)

// NewClient returns an Anthropic client. When model is empty it uses the default
// model. The apiKey may be empty: Configured reports that so callers can return
// a clear "not configured" response instead of a failing API call.
func NewClient(apiKey, model string) *Client {
	if model == "" {
		model = defaultModel
	}
	return &Client{
		apiKey: apiKey,
		model:  model,
		http:   &http.Client{Timeout: requestTimeout},
	}
}

// Configured reports whether an API key is set, so the usecase can short-circuit
// to a structured "AI is not configured" reply rather than calling the API.
func (c *Client) Configured() bool { return c.apiKey != "" }

// anthropic request/response shapes, only the fields mountabo uses.
type messageRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	System    []systemBlock `json:"system"`
	Messages  []chatMessage `json:"messages"`
}

type systemBlock struct {
	Type         string        `json:"type"`
	Text         string        `json:"text"`
	CacheControl *cacheControl `json:"cache_control,omitempty"`
}

type cacheControl struct {
	Type string `json:"type"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type messageResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// Suggest asks the model for a command for the user's request. userContext is
// optional free text (e.g. the server's OS or recent output) that helps the
// model tailor the command; it is appended to the user turn when present.
func (c *Client) Suggest(ctx context.Context, prompt, userContext string) (usecase.CommandSuggestion, error) {
	reqBody := buildRequest(c.model, prompt, userContext)
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return usecase.CommandSuggestion{}, fmt.Errorf("encode request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, messagesURL, bytes.NewReader(payload))
	if err != nil {
		return usecase.CommandSuggestion{}, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", apiVersion)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return usecase.CommandSuggestion{}, fmt.Errorf("call anthropic: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return usecase.CommandSuggestion{}, fmt.Errorf("read response: %w", err)
	}

	var parsed messageResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return usecase.CommandSuggestion{}, fmt.Errorf("decode response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		if parsed.Error != nil {
			return usecase.CommandSuggestion{}, fmt.Errorf("anthropic error: %s", parsed.Error.Message)
		}
		return usecase.CommandSuggestion{}, fmt.Errorf("anthropic returned status %d", resp.StatusCode)
	}

	text := joinText(parsed)
	suggestion, err := parseSuggestion(text)
	if err != nil {
		return usecase.CommandSuggestion{}, err
	}
	return suggestion, nil
}

// buildRequest assembles the Messages payload. The system prompt is a cached
// block (cache_control: ephemeral): it is constant across requests, so after the
// first call the model reads it from cache, cutting input cost and latency.
func buildRequest(model, prompt, userContext string) messageRequest {
	content := "Request: " + strings.TrimSpace(prompt)
	if ctx := strings.TrimSpace(userContext); ctx != "" {
		content += "\n\nServer context: " + ctx
	}
	return messageRequest{
		Model:     model,
		MaxTokens: maxTokens,
		System: []systemBlock{{
			Type:         "text",
			Text:         systemPrompt,
			CacheControl: &cacheControl{Type: "ephemeral"},
		}},
		Messages: []chatMessage{{Role: "user", Content: content}},
	}
}

// joinText concatenates the text blocks of the model's reply.
func joinText(r messageResponse) string {
	var b strings.Builder
	for _, block := range r.Content {
		if block.Type == "text" {
			b.WriteString(block.Text)
		}
	}
	return b.String()
}

// parseSuggestion pulls the {command, explanation} JSON out of the model's text.
// The model is instructed to return bare JSON, but it may wrap it in a markdown
// fence, so this strips a fence and locates the first JSON object before
// decoding, rather than trusting the reply to be exactly clean.
func parseSuggestion(text string) (usecase.CommandSuggestion, error) {
	raw := extractJSON(text)
	if raw == "" {
		return usecase.CommandSuggestion{}, fmt.Errorf("model returned no parseable suggestion")
	}
	var out usecase.CommandSuggestion
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return usecase.CommandSuggestion{}, fmt.Errorf("decode suggestion: %w", err)
	}
	out.Command = strings.TrimSpace(out.Command)
	out.Explanation = strings.TrimSpace(out.Explanation)
	return out, nil
}

// extractJSON returns the first {...} object found in text, tolerating a
// surrounding markdown code fence or stray prose.
func extractJSON(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	start := strings.IndexByte(text, '{')
	end := strings.LastIndexByte(text, '}')
	if start < 0 || end < start {
		return ""
	}
	return text[start : end+1]
}
