// Package embed implements the optional embeddings path decided in
// https://github.com/cheetahbyte/lore/issues/7: vector search is opt-in,
// never generated in-process (no bundled/local model, per the research on
// issue #6) — always an external API call.
package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Embedder turns text into vectors via an external service.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// OpenAICompatible talks to any embeddings endpoint that speaks the OpenAI
// /v1/embeddings request/response shape — this covers both configured
// providers from issue #3 (config.toml's embeddings.provider: "openai" or
// "ollama", since Ollama serves an OpenAI-compatible endpoint).
type OpenAICompatible struct {
	HTTPClient *http.Client
	Endpoint   string // e.g. https://api.openai.com/v1 or http://localhost:11434/v1
	APIKey     string
	Model      string
}

func New(endpoint, apiKey, model string) *OpenAICompatible {
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "text-embedding-3-small"
	}
	return &OpenAICompatible{HTTPClient: http.DefaultClient, Endpoint: endpoint, APIKey: apiKey, Model: model}
}

type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

func (o *OpenAICompatible) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(embedRequest{Model: o.Model, Input: texts})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.Endpoint+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if o.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.APIKey)
	}
	resp, err := o.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed: request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embed: unexpected status %d", resp.StatusCode)
	}
	var out embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("embed: decode response: %w", err)
	}
	vectors := make([][]float32, len(texts))
	for _, d := range out.Data {
		if d.Index >= 0 && d.Index < len(vectors) {
			vectors[d.Index] = d.Embedding
		}
	}
	return vectors, nil
}
