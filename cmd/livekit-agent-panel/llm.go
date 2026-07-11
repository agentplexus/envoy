package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// LLMClient is the interface for generating LLM responses.
type LLMClient interface {
	Generate(ctx context.Context, systemPrompt, userMessage string, maxTokens int) (string, error)
}

// NewLLMClient creates an LLM client for the given provider.
func NewLLMClient(provider, apiKey, model string) LLMClient {
	switch provider {
	case "openai":
		return &OpenAIClient{apiKey: apiKey, model: model}
	default: // "anthropic"
		return &AnthropicClient{apiKey: apiKey, model: model}
	}
}

// AnthropicClient implements LLMClient for Anthropic's API.
type AnthropicClient struct {
	apiKey string
	model  string
}

// Generate generates a response using Anthropic's API.
func (c *AnthropicClient) Generate(ctx context.Context, systemPrompt, userMessage string, maxTokens int) (string, error) {
	payload := map[string]any{
		"model":      c.model,
		"max_tokens": maxTokens,
		"system":     systemPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": userMessage},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.anthropic.com/v1/messages",
		bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("anthropic error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("no response from Anthropic")
	}

	return result.Content[0].Text, nil
}

// OpenAIClient implements LLMClient for OpenAI's API.
type OpenAIClient struct {
	apiKey string
	model  string
}

// Generate generates a response using OpenAI's API.
func (c *OpenAIClient) Generate(ctx context.Context, systemPrompt, userMessage string, maxTokens int) (string, error) {
	messages := []map[string]string{
		{"role": "user", "content": userMessage},
	}
	if systemPrompt != "" {
		messages = []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userMessage},
		}
	}

	payload := map[string]any{
		"model":      c.model,
		"max_tokens": maxTokens,
		"messages":   messages,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.openai.com/v1/chat/completions",
		bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openai error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return result.Choices[0].Message.Content, nil
}
