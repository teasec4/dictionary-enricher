package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// LLMConfig holds configuration for the LLM API client.
type LLMConfig struct {
	BaseURL string // e.g. "http://localhost:1234"
	Model   string // e.g. "gemma-4"
	Timeout time.Duration
}

// LLMClient wraps an HTTP client for OpenAI-compatible chat completions.
type LLMClient struct {
	config LLMConfig
	client *http.Client
}

// NewLLMClient creates a new LLM client with the given configuration.
func NewLLMClient(cfg LLMConfig) *LLMClient {
	if cfg.Timeout == 0 {
		cfg.Timeout = 120 * time.Second
	}
	return &LLMClient{
		config: cfg,
		client: &http.Client{Timeout: cfg.Timeout},
	}
}

// ChatJSON sends a chat completion request and parses the response into the provided target.
// target must be a pointer to a JSON-unmarshalable value.
func (c *LLMClient) ChatJSON(ctx context.Context, systemPrompt, userPrompt string, temperature float64, maxTokens int, target any) error {
	reqBody := map[string]any{
		"model": c.config.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": temperature,
		"max_tokens":  maxTokens,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.BaseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("LLM API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return fmt.Errorf("parse chat response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return fmt.Errorf("no choices in LLM response")
	}

	content := chatResp.Choices[0].Message.Content
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	if err := json.Unmarshal([]byte(content), target); err != nil {
		return fmt.Errorf("parse LLM JSON output: %w\nRaw: %s", err, content[:min(len(content), 500)])
	}

	return nil
}

// GenerateExamplesBatch sends a batch of words to the LLM and parses the response.
// Returns one ExampleResult per word, each with 2 examples (everyday + literary).
func (c *LLMClient) GenerateExamplesBatch(ctx context.Context, words []string) ([]ExampleResult, error) {
	if len(words) == 0 {
		return nil, nil
	}

	wordList := strings.Join(words, ", ")

	prompt := fmt.Sprintf(`Ты — лингвист-синолог. Для каждого китайского слова из списка напиши ДВА примера предложения на русском языке (кириллицей) — без пиньиня и без иероглифов.

Первый пример — бытовой, повседневный контекст.
Второй пример — специальный или литературный контекст (книжный/поэтический/абстрактный).

Список слов: %s

Ответь строго в формате JSON — массив объектов:
[
  {"headword": "слово", "examples": ["пример 1", "пример 2"]},
  {"headword": "слово2", "examples": ["пример 1", "пример 2"]}
]

Только JSON, без пояснений и markdown-обёртки.`, wordList)

	reqBody := map[string]any{
		"model": c.config.Model,
		"messages": []map[string]string{
			{"role": "system", "content": "Ты — лингвист-синолог, эксперт по китайскому языку. Отвечай строго в формате JSON."},
			{"role": "user", "content": prompt},
		},
		"temperature": 0.7,
		"max_tokens":  4096,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.BaseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse OpenAI-style response
	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("parse chat response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in LLM response")
	}

	content := chatResp.Choices[0].Message.Content
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var results []ExampleResult
	if err := json.Unmarshal([]byte(content), &results); err != nil {
		return nil, fmt.Errorf("parse LLM JSON output: %w\nRaw: %s", err, content[:min(len(content), 500)])
	}

	return results, nil
}
