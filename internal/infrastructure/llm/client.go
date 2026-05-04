package llm

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

type Config struct {
	BaseURL string
	Model   string
	Timeout time.Duration
}

type Client struct {
	config Config
	http   *http.Client
}

func NewClient(cfg Config) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = 120 * time.Second
	}
	return &Client{
		config: cfg,
		http:   &http.Client{Timeout: cfg.Timeout},
	}
}

// ChatJSON отправляет chat completion и парсит JSON-ответ в target.
// target должен быть указателем (напр. &CleanResult{}).
// Автоматически чистит ```json ... ``` обёртку.
// Пытается восстановить JSON при типовых ошибках Gemma 4.
func (c *Client) ChatJSON(ctx context.Context, userPrompt string,
	 target any) error {

	reqBody := map[string]any{
		"messages": []map[string]string{
			{"role": "system", "content": ExamplePrompt()},
			{"role": "user", "content": userPrompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(c.config.BaseURL, "/")
	if !strings.HasSuffix(url, "/chat/completions") {
		url += "/chat/completions"
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("LLM status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return fmt.Errorf("unmarshal chat response: %w (body: %s)", err, string(respBody))
	}

	if len(chatResp.Choices) == 0 {
		return fmt.Errorf("no choices in response")
	}

	content := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	// чистим ```json ... ``` обёртку
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	// Пробуем распарсить как есть
	if err := json.Unmarshal([]byte(content), target); err != nil {
		return fmt.Errorf("unmarshal target: %w\nraw content: %s", err, content)
	}

	return nil
}

