package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Native (Ollama/OpenRouter) conversations are persisted per tab as JSON under
// ~/.tiramisu/sessions/agent/<tabID>.json so they survive app restarts. Claude
// sessions are persisted by the claude CLI itself; this is the native analogue.

func (a *App) agentHistoryPath(tabID string) (string, error) {
	if tabID == "" || strings.ContainsAny(tabID, "/\\") || strings.Contains(tabID, "..") {
		return "", fmt.Errorf("invalid tab id %q", tabID)
	}
	dir, err := a.sessionsDir()
	if err != nil {
		return "", err
	}
	agentDir := filepath.Join(dir, "agent")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(agentDir, tabID+".json"), nil
}

func (a *App) saveAgentHistory(tabID string, history []ChatTurn) {
	path, err := a.agentHistoryPath(tabID)
	if err != nil {
		return
	}
	data, err := json.Marshal(history)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0644)
}

func (a *App) loadAgentHistory(tabID string) ([]ChatTurn, error) {
	path, err := a.agentHistoryPath(tabID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var h []ChatTurn
	if err := json.Unmarshal(data, &h); err != nil {
		return nil, err
	}
	return h, nil
}

// DeleteAgentHistory removes a tab's persisted native conversation (bound).
func (a *App) DeleteAgentHistory(tabID string) error {
	path, err := a.agentHistoryPath(tabID)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// LoadAgentSessionHistory returns a tab's persisted native conversation as renderable
// HistoryMessages (reusing the Claude history shape so the frontend renders both).
func (a *App) LoadAgentSessionHistory(tabID string) ([]HistoryMessage, error) {
	history, err := a.loadAgentHistory(tabID)
	if err != nil || len(history) == 0 {
		return nil, err
	}
	var out []HistoryMessage
	toolByID := map[string]*HistoryTool{}
	for _, turn := range history {
		switch turn.Role {
		case "user":
			out = append(out, HistoryMessage{Role: "user", Content: turn.Content})
		case "assistant":
			msg := HistoryMessage{Role: "assistant", Content: turn.Content}
			for _, tc := range turn.ToolCalls {
				msg.Tools = append(msg.Tools, HistoryTool{ID: tc.ID, Name: tc.Function.Name, Input: tc.Function.Arguments})
			}
			out = append(out, msg)
			last := &out[len(out)-1]
			for i := range last.Tools {
				toolByID[last.Tools[i].ID] = &last.Tools[i]
			}
		case "tool":
			if ht, ok := toolByID[turn.ToolCallID]; ok {
				ht.Output = turn.Content
			}
		}
	}
	return out, nil
}
