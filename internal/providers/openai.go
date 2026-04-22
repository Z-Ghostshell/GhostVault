package providers

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

type OpenAIClient struct {
	BaseURL       string
	EmbeddingsURL string // optional override; empty => OpenAI BaseURL + "/embeddings"
	APIKey        string
	HTTP          *http.Client
}

func NewOpenAI(base, apiKey string) *OpenAIClient {
	return &OpenAIClient{
		BaseURL: strings.TrimRight(base, "/"),
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *OpenAIClient) embeddingsEndpoint() string {
	if u := strings.TrimSpace(c.EmbeddingsURL); u != "" {
		return strings.TrimRight(u, "/")
	}
	return c.BaseURL + "/embeddings"
}

func (c *OpenAIClient) Embed(ctx context.Context, model string, texts []string) ([][]float32, error) {
	body := map[string]any{"model": model, "input": texts}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	ep := c.embeddingsEndpoint()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embeddings POST %s: %s: %s", ep, resp.Status, string(b))
	}
	var out struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	res := make([][]float32, len(out.Data))
	for i, d := range out.Data {
		v := make([]float32, len(d.Embedding))
		for j, x := range d.Embedding {
			v[j] = float32(x)
		}
		res[i] = v
	}
	return res, nil
}

const extractionSystem = `You extract stable, factual memories from conversation. Output JSON only: {"memory":["fact1", ...]} additive only, no updates or deletes. Empty list if nothing to store.`

func (c *OpenAIClient) ExtractMemories(ctx context.Context, model, userPayload string) ([]string, error) {
	body := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": extractionSystem},
			{"role": "user", "content": userPayload},
		},
		"temperature": 0.2,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("chat: %s: %s", resp.Status, string(b))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	if len(out.Choices) == 0 {
		return nil, nil
	}
	content := strings.TrimSpace(out.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	var parsed struct {
		Memory []string `json:"memory"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, fmt.Errorf("parse extraction json: %w", err)
	}
	return parsed.Memory, nil
}
