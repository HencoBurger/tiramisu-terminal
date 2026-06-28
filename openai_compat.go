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
// OpenRouter both speak /v1/chat/completions (SSE), incl. tool calling; they differ
// only in base URL, auth header, and the model-listing endpoint. Stdlib only.
type openAICompat struct {
	chatURL          string
	apiKey           string
	listURL          string
	listKind         string // "ollama" | "openai"
	disableReasoning bool   // send reasoning_effort:none to suppress thinking
	client           *http.Client
}

func newOpenAICompat(base, apiKey, listURL, listKind string, disableReasoning bool) *openAICompat {
	return &openAICompat{
		chatURL:          strings.TrimRight(base, "/") + "/chat/completions",
		apiKey:           apiKey,
		listURL:          listURL,
		listKind:         listKind,
		disableReasoning: disableReasoning,
		client:           &http.Client{},
	}
}

type oaToolDef struct {
	Type     string `json:"type"`
	Function struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Parameters  map[string]interface{} `json:"parameters"`
	} `json:"function"`
}

type oaChatRequest struct {
	Model           string        `json:"model"`
	Messages        []interface{} `json:"messages"`
	Stream          bool          `json:"stream"`
	Tools           []oaToolDef   `json:"tools,omitempty"`
	ReasoningEffort string        `json:"reasoning_effort,omitempty"` // Ollama: "none" to skip thinking
	Reasoning       interface{}   `json:"reasoning,omitempty"`        // OpenRouter: {exclude:true}
}

// toOAMessage converts a ChatTurn to an OpenAI message. Turns with images become
// multimodal content (an array of text + image_url parts); everything else stays a
// plain string-content message.
func toOAMessage(t ChatTurn) interface{} {
	if len(t.Images) > 0 {
		parts := make([]interface{}, 0, len(t.Images)+1)
		if t.Content != "" {
			parts = append(parts, map[string]interface{}{"type": "text", "text": t.Content})
		}
		for _, img := range t.Images {
			parts = append(parts, map[string]interface{}{
				"type":      "image_url",
				"image_url": map[string]string{"url": img},
			})
		}
		return map[string]interface{}{"role": t.Role, "content": parts}
	}
	m := map[string]interface{}{"role": t.Role, "content": t.Content}
	if len(t.ToolCalls) > 0 {
		m["tool_calls"] = t.ToolCalls
	}
	if t.ToolCallID != "" {
		m["tool_call_id"] = t.ToolCallID
	}
	return m
}

type oaChatChunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			Reasoning string `json:"reasoning"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func toolDefs(tools []Tool) []oaToolDef {
	defs := make([]oaToolDef, 0, len(tools))
	for _, t := range tools {
		var d oaToolDef
		d.Type = "function"
		d.Function.Name = t.Name
		d.Function.Description = t.Description
		d.Function.Parameters = t.Parameters
		defs = append(defs, d)
	}
	return defs
}

func (c *openAICompat) StreamChat(ctx context.Context, model string, messages []ChatTurn, tools []Tool, cb StreamCallbacks) (StreamResult, error) {
	msgs := make([]interface{}, len(messages))
	for i, t := range messages {
		msgs[i] = toOAMessage(t)
	}
	reqBody := oaChatRequest{Model: model, Messages: msgs, Stream: true}
	if len(tools) > 0 {
		reqBody.Tools = toolDefs(tools)
	}
	if c.disableReasoning {
		// Ollama understands reasoning_effort:none; OpenRouter uses reasoning:{exclude:true}.
		if c.listKind == "ollama" {
			reqBody.ReasoningEffort = "none"
		} else {
			reqBody.Reasoning = map[string]interface{}{"exclude": true}
		}
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return StreamResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.chatURL, bytes.NewReader(body))
	if err != nil {
		return StreamResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return StreamResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return StreamResult{}, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(b)))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var content strings.Builder
	var reasoning strings.Builder
	acc := map[int]*ToolCall{}
	var order []int
	finish := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
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
			return StreamResult{}, fmt.Errorf("%s", chunk.Error.Message)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		ch := chunk.Choices[0]
		if ch.Delta.Content != "" {
			content.WriteString(ch.Delta.Content)
			if cb.OnText != nil {
				cb.OnText(ch.Delta.Content)
			}
		}
		if ch.Delta.Reasoning != "" {
			reasoning.WriteString(ch.Delta.Reasoning)
			if cb.OnReasoning != nil {
				cb.OnReasoning(ch.Delta.Reasoning)
			}
		}
		for _, tc := range ch.Delta.ToolCalls {
			cur, ok := acc[tc.Index]
			if !ok {
				cur = &ToolCall{Type: "function"}
				acc[tc.Index] = cur
				order = append(order, tc.Index)
			}
			if tc.ID != "" {
				cur.ID = tc.ID
			}
			nameWasEmpty := cur.Function.Name == ""
			if tc.Function.Name != "" {
				cur.Function.Name = tc.Function.Name
			}
			if nameWasEmpty && cur.Function.Name != "" && cb.OnToolStart != nil {
				cb.OnToolStart(tc.Index, cur.ID, cur.Function.Name)
			}
			if tc.Function.Arguments != "" {
				cur.Function.Arguments += tc.Function.Arguments
				if cb.OnToolArgs != nil {
					cb.OnToolArgs(tc.Index, tc.Function.Arguments)
				}
			}
		}
		if ch.FinishReason != nil {
			finish = *ch.FinishReason
		}
	}
	if err := scanner.Err(); err != nil {
		return StreamResult{}, err
	}

	res := StreamResult{Content: content.String(), Reasoning: reasoning.String(), Finish: finish}
	for _, idx := range order {
		res.ToolCalls = append(res.ToolCalls, *acc[idx])
	}
	return res, nil
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
