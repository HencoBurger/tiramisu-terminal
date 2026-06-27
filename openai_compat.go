package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// openAICompat is one OpenAI-compatible client used by BOTH providers. Ollama and
// OpenRouter both speak /v1/chat/completions (SSE); they differ only in base URL,
// auth header, and the model-listing endpoint. Stdlib only — no new deps.
type openAICompat struct {
	chatURL  string // {base}/chat/completions
	apiKey   string // "" => no Authorization header (Ollama)
	listURL  string // ollama: {base}/api/tags ; openrouter: /v1/models
	listKind string // "ollama" | "openai"
	client   *http.Client
}

func newOpenAICompat(base, apiKey, listURL, listKind string) *openAICompat {
	return &openAICompat{
		chatURL:  strings.TrimRight(base, "/") + "/chat/completions",
		apiKey:   apiKey,
		listURL:  listURL,
		listKind: listKind,
		client:   &http.Client{},
	}
}

type oaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type oaChatRequest struct {
	Model    string      `json:"model"`
	Messages []oaMessage `json:"messages"`
	Stream   bool        `json:"stream"`
}

type oaChatChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *openAICompat) StreamChat(ctx context.Context, model string, messages []ChatTurn, onDelta func(string)) (string, error) {
	msgs := make([]oaMessage, len(messages))
	for i, m := range messages {
		msgs[i] = oaMessage{Role: m.Role, Content: m.Content}
	}
	body, err := json.Marshal(oaChatRequest{Model: model, Messages: msgs, Stream: true})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.chatURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(b)))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	finishReason := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip blanks and SSE comment/keep-alive lines (e.g. OpenRouter ": PROCESSING").
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk oaChatChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil {
			return finishReason, fmt.Errorf("%s", chunk.Error.Message)
		}
		if len(chunk.Choices) > 0 {
			if t := chunk.Choices[0].Delta.Content; t != "" {
				onDelta(t)
			}
			if fr := chunk.Choices[0].FinishReason; fr != nil {
				finishReason = *fr
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return finishReason, err
	}
	return finishReason, nil
}

func (c *openAICompat) ListModels(ctx context.Context) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.listURL, nil)
	if err != nil {
		return nil, err
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if c.listKind == "ollama" {
		var out struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, err
		}
		models := make([]ModelInfo, 0, len(out.Models))
		for _, m := range out.Models {
			models = append(models, ModelInfo{ID: m.Name, Name: m.Name})
		}
		return models, nil
	}

	// openai-style /v1/models
	var out struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	models := make([]ModelInfo, 0, len(out.Data))
	for _, m := range out.Data {
		name := m.Name
		if name == "" {
			name = m.ID
		}
		models = append(models, ModelInfo{ID: m.ID, Name: name})
	}
	return models, nil
}
