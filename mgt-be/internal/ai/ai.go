// Package ai is the server-side LLM client used by /pulls/{n}/describe and
// the AI-fill flow on submit. Configured via env (LLM_API_KEY, LLM_BASE_URL,
// LLM_MODEL); ports the logic that used to live in mgt-cli/pkg/ai.
package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const DefaultPRTemplate = `## Summary

<!-- What does this PR do and why? -->

## Changes

<!-- List the key changes -->

## Test Plan

<!-- How was this tested? -->
`

var ErrNotConfigured = errors.New("ai: LLM not configured")

type Client struct {
	apiKey  string
	baseURL string
	model   string
	http    *http.Client
}

func New(apiKey, baseURL, model string) *Client {
	return &Client{apiKey: apiKey, baseURL: baseURL, model: model, http: http.DefaultClient}
}

func (c *Client) Configured() bool { return c.apiKey != "" }

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
}
type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type chatResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *Client) complete(system, user string) (string, error) {
	if !c.Configured() {
		return "", ErrNotConfigured
	}
	body, err := json.Marshal(chatRequest{
		Model: c.model,
		Messages: []message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var out chatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("LLM response parse: %w", err)
	}
	if out.Error != nil {
		return "", fmt.Errorf("LLM API: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no choices")
	}
	return out.Choices[0].Message.Content, nil
}

// FillPRBody takes the PR template plus the branch's diff and commits and
// returns a filled-in description.
func (c *Client) FillPRBody(template, diff, commits string) (string, error) {
	return c.complete(
		"You are a concise pull request description writer. "+
			"Output ONLY the filled PR template, no markdown fences wrapping the output, no extra commentary.",
		fmt.Sprintf(
			"Here is the PR description template:\n```\n%s\n```\n\n"+
				"Here are the commits on this branch:\n```\n%s\n```\n\n"+
				"Here is the diff:\n```\n%s\n```\n\n"+
				"Fill in the template based on the commits and diff. Be concise and specific. Output only the filled template. Fill the output as per template, dont make up new sections",
			template, truncate(commits, 2000), truncate(diff, 10000)),
	)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n... (truncated)"
}
