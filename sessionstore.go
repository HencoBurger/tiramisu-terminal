package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type StoredSession struct {
	SessionID  string `json:"sessionId"`
	ProjectDir string `json:"projectDir"`
	FirstPrompt string `json:"firstPrompt"`
	MessageCount int   `json:"messageCount"`
	LastModified int64  `json:"lastModified"`
}

func (a *App) ListClaudeSessions() ([]StoredSession, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	projectsDir := filepath.Join(home, ".claude", "projects")
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		return nil, nil
	}

	var sessions []StoredSession

	err = filepath.Walk(projectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}

		if info.IsDir() || !strings.HasSuffix(info.Name(), ".jsonl") {
			return nil
		}

		session := parseSessionFile(path, projectsDir)
		if session != nil {
			sessions = append(sessions, *session)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastModified > sessions[j].LastModified
	})

	return sessions, nil
}

func parseSessionFile(path, projectsDir string) *StoredSession {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		return nil
	}

	sessionID := strings.TrimSuffix(filepath.Base(path), ".jsonl")

	// Get project dir from path relative to projects dir
	relPath, _ := filepath.Rel(projectsDir, filepath.Dir(path))
	projectDir := decodeProjectPath(relPath)

	info, _ := os.Stat(path)
	lastMod := time.Now().Unix()
	if info != nil {
		lastMod = info.ModTime().Unix()
	}

	firstPrompt := ""
	messageCount := 0

	for _, line := range lines {
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		msgType, _ := raw["type"].(string)
		if msgType != "user" && msgType != "assistant" {
			continue
		}
		messageCount++

		if firstPrompt == "" && msgType == "user" {
			message, _ := raw["message"].(map[string]interface{})
			if message == nil {
				continue
			}
			content := message["content"]
			if s, ok := content.(string); ok {
				firstPrompt = s
			} else if arr, ok := content.([]interface{}); ok {
				for _, c := range arr {
					if cMap, ok := c.(map[string]interface{}); ok {
						if text, ok := cMap["text"].(string); ok {
							firstPrompt = text
							break
						}
					}
				}
			}
			if len(firstPrompt) > 200 {
				firstPrompt = firstPrompt[:200] + "..."
			}
		}
	}

	return &StoredSession{
		SessionID:    sessionID,
		ProjectDir:   projectDir,
		FirstPrompt:  firstPrompt,
		MessageCount: messageCount,
		LastModified:  lastMod,
	}
}

// HistoryMessage represents a message from a stored session for the frontend.
type HistoryMessage struct {
	Role    string          `json:"role"`    // "user" or "assistant"
	Content string          `json:"content"` // text content
	Tools   []HistoryTool   `json:"tools"`   // tool uses (assistant) or tool results (user)
}

type HistoryTool struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Input  string `json:"input"`
	Output string `json:"output"`
}

// LoadSessionHistory reads a Claude CLI session .jsonl file and returns
// the conversation messages in a format the frontend can display.
func (a *App) LoadSessionHistory(sessionID, projectDir string) ([]HistoryMessage, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	projectsDir := filepath.Join(home, ".claude", "projects")

	// Find the session file by walking project dirs
	var sessionPath string
	_ = filepath.Walk(projectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.TrimSuffix(info.Name(), ".jsonl") == sessionID {
			sessionPath = path
			return filepath.SkipAll
		}
		return nil
	})

	if sessionPath == "" {
		return nil, fmt.Errorf("session file not found for %s", sessionID)
	}

	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var messages []HistoryMessage

	// Track tool uses by ID so we can match results
	toolMap := make(map[string]*HistoryTool)

	for _, line := range lines {
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		// Top-level "type" field is "user" or "assistant"
		// Actual content is inside "message" object
		msgType, _ := raw["type"].(string)
		if msgType != "user" && msgType != "assistant" {
			continue
		}

		message, _ := raw["message"].(map[string]interface{})
		if message == nil {
			continue
		}

		histMsg := HistoryMessage{
			Role: msgType,
		}

		// Content can be a string or array of content blocks
		content := message["content"]
		switch c := content.(type) {
		case string:
			histMsg.Content = c
		case []interface{}:
			for _, block := range c {
				bMap, ok := block.(map[string]interface{})
				if !ok {
					continue
				}
				blockType, _ := bMap["type"].(string)
				switch blockType {
				case "text":
					text, _ := bMap["text"].(string)
					histMsg.Content += text
				case "tool_use":
					id, _ := bMap["id"].(string)
					name, _ := bMap["name"].(string)
					var inputStr string
					if input, ok := bMap["input"]; ok {
						inputBytes, _ := json.Marshal(input)
						inputStr = string(inputBytes)
					}
					tool := HistoryTool{
						ID:    id,
						Name:  name,
						Input: inputStr,
					}
					histMsg.Tools = append(histMsg.Tools, tool)
					toolMap[id] = &histMsg.Tools[len(histMsg.Tools)-1]
				case "tool_result":
					toolID, _ := bMap["tool_use_id"].(string)
					var resultText string
					switch rc := bMap["content"].(type) {
					case string:
						resultText = rc
					case []interface{}:
						for _, part := range rc {
							if pMap, ok := part.(map[string]interface{}); ok {
								if t, ok := pMap["text"].(string); ok {
									resultText += t
								}
							}
						}
					}
					// Match back to the tool use
					if t, ok := toolMap[toolID]; ok {
						t.Output = resultText
					}
				}
			}
		}

		messages = append(messages, histMsg)
	}

	return messages, nil
}

func decodeProjectPath(encoded string) string {
	// Claude encodes paths by replacing / with - (on Linux/Mac)
	// This is a best-effort decode
	return "/" + strings.ReplaceAll(encoded, "-", "/")
}
