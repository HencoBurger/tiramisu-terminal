package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ModelInfo is a provider-neutral model descriptor for the picker.
type ModelInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// StreamCallbacks receive streaming deltas as a provider response arrives.
type StreamCallbacks struct {
	OnText      func(text string)
	OnReasoning func(text string) // reasoning-model "thinking" deltas
	OnToolStart func(index int, id, name string)
	OnToolArgs  func(index int, argsDelta string)
}

// StreamResult is the assembled outcome of one provider turn.
type StreamResult struct {
	Content   string
	Reasoning string
	ToolCalls []ToolCall
	Finish    string
}

// Provider is the seam every model backend implements.
type Provider interface {
	StreamChat(ctx context.Context, model string, messages []ChatTurn, tools []Tool, cb StreamCallbacks) (StreamResult, error)
	ListModels(ctx context.Context) ([]ModelInfo, error)
}

const defaultOllamaBaseURL = "http://localhost:11434"

// providerFor resolves a provider by name at request time, pulling config + secrets.
func (a *App) providerFor(name string) (Provider, error) {
	switch name {
	case "ollama":
		base := strings.TrimRight(a.globalConfig.OllamaBaseURL, "/")
		if base == "" {
			base = defaultOllamaBaseURL
		}
		return newOpenAICompat(base+"/v1", "", base+"/api/tags", "ollama", a.globalConfig.DisableThinking), nil
	case "openrouter":
		key := a.resolveProviderKey("openrouter")
		if key == "" {
			return nil, fmt.Errorf("OpenRouter API key not set — add it in Settings")
		}
		return newOpenAICompat("https://openrouter.ai/api/v1", key, "https://openrouter.ai/api/v1/models", "openai", a.globalConfig.DisableThinking), nil
	default:
		return nil, fmt.Errorf("unknown provider %q", name)
	}
}

// ListProviderModels returns the models offered by a provider (bound to the frontend).
func (a *App) ListProviderModels(provider string) ([]ModelInfo, error) {
	prov, err := a.providerFor(provider)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return prov.ListModels(ctx)
}
