package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/santhosh/mgt/pkg/config"
)

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

func complete(system, user string) (string, error) {
	key := config.OpenAIKey()
	if key == "" {
		return "", fmt.Errorf("OpenAI API key not set. Set OPENAI_API_KEY env or run: mgt config set openai_key <key>")
	}

	reqBody := chatRequest{
		Model: "gpt-4o-mini",
		Messages: []message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenAI request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse OpenAI response: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("OpenAI API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("OpenAI returned no choices")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// FillPRBody fills a PR template using the branch diff and commit log.
func FillPRBody(template, diff, commitLog string) (string, error) {
	return complete(
		"You are a concise pull request description writer. "+
			"Output ONLY the filled PR template, no markdown fences wrapping the output, no extra commentary.",
		fmt.Sprintf(
			"Here is the PR description template:\n```\n%s\n```\n\n"+
				"Here are the commits on this branch:\n```\n%s\n```\n\n"+
				"Here is the diff:\n```\n%s\n```\n\n"+
				"Fill in the template based on the commits and diff. Be concise and specific. Output only the filled template. Fill the output as per template, dont make up new sections",
			template, truncate(commitLog, 2000), truncate(diff, 10000)),
	)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n... (truncated)"
}
