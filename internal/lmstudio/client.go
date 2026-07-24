package lmstudio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	model   string
	apiKey  string
	http    *http.Client
}

// New builds a client for any OpenAI-compatible endpoint. apiKey may be empty
// for keyless backends like LM Studio; when set it is sent as a Bearer token
// (required by hosted providers such as MiniMax).
func New(baseURL, model, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		model:   model,
		apiKey:  apiKey,
		// Quality > speed — a long transcript can take several minutes
		http: &http.Client{Timeout: 20 * time.Minute},
	}
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type completionRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Temperature float64   `json:"temperature"`
	Stream      bool      `json:"stream"`
}

type completionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Complete sends a system + user prompt and returns the model's response.
func (c *Client) Complete(systemPrompt, userPrompt string) (string, error) {
	reqBody := completionRequest{
		Model: c.model,
		Messages: []message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.3, // low temp for factual extraction
		Stream:      false,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(
		http.MethodPost,
		c.baseURL+"/v1/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("POST to LLM: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM returned %s: %s", resp.Status, raw)
	}

	var result completionResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("LM Studio returned no choices")
	}

	return result.Choices[0].Message.Content, nil
}
