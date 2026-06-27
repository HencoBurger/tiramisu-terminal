package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

// Native provider-agnostic chat runtime. Mirrors session.go's lifecycle shape (a map
// keyed by tabID + per-session cancel/done), but talks to provider HTTP APIs instead
// of spawning a CLI. The Claude path (session.go) is untouched.

const agentSystemPrompt = "You are a helpful coding assistant integrated into the Tiramisu desktop app. Answer clearly and concisely. Use Markdown formatting, with fenced code blocks for code."

// ChatTurn is one message in a conversation (provider-neutral).
type ChatTurn struct {
	Role    string `json:"role"` // system | user | assistant  (M1 adds: tool)
	Content string `json:"content"`
}

// AgentEvent is the normalized streaming event emitted on the "agent:event" channel.
// M1+ extends this with tool/agent fields (additive).
type AgentEvent struct {
	Type  string `json:"type"`            // message_start | text_delta | done | error
	Text  string `json:"text,omitempty"`  // text_delta payload
	Error string `json:"error,omitempty"` // error payload
}

type AgentSession struct {
	TabID    string
	Provider string
	Model    string
	WorkDir  string // unused in M0; threaded now for M1 tools
	cancel   context.CancelFunc
	done     chan struct{}
	mu       sync.Mutex
	history  []ChatTurn
}

// AgentStart begins a fresh native-provider conversation in a tab.
func (a *App) AgentStart(tabID, provider, model, workDir, prompt string) error {
	a.stopAgentForTab(tabID)
	history := []ChatTurn{
		{Role: "system", Content: agentSystemPrompt},
		{Role: "user", Content: prompt},
	}
	a.startAgentRun(tabID, provider, model, workDir, history)
	return nil
}

// AgentSend continues a tab's native conversation. Self-heals to a fresh start if the
// tab isn't tracked (after an app restart or a provider switch) — never hard-errors.
func (a *App) AgentSend(tabID, provider, model, workDir, prompt string) error {
	a.agentMu.RLock()
	session, ok := a.agentSessions[tabID]
	a.agentMu.RUnlock()
	if !ok {
		return a.AgentStart(tabID, provider, model, workDir, prompt)
	}

	// Stop the prior run (keeping its accumulated history), then append + re-run.
	a.stopAgentForTab(tabID)
	session.mu.Lock()
	history := append(append([]ChatTurn{}, session.history...), ChatTurn{Role: "user", Content: prompt})
	session.mu.Unlock()
	a.startAgentRun(tabID, provider, model, workDir, history)
	return nil
}

// AgentStop cancels and removes a tab's native conversation.
func (a *App) AgentStop(tabID string) error {
	a.stopAgentForTab(tabID)
	return nil
}

func (a *App) startAgentRun(tabID, provider, model, workDir string, history []ChatTurn) {
	ctx, cancel := context.WithCancel(context.Background())
	session := &AgentSession{
		TabID:    tabID,
		Provider: provider,
		Model:    model,
		WorkDir:  workDir,
		cancel:   cancel,
		done:     make(chan struct{}),
		history:  history,
	}
	a.agentMu.Lock()
	a.agentSessions[tabID] = session
	a.agentMu.Unlock()

	go a.runAgent(ctx, session)
}

func (a *App) stopAgentForTab(tabID string) {
	a.agentMu.Lock()
	session, ok := a.agentSessions[tabID]
	if ok {
		delete(a.agentSessions, tabID)
	}
	a.agentMu.Unlock()
	if ok {
		a.stopAgent(session)
	}
}

func (a *App) stopAgent(session *AgentSession) {
	if session.cancel != nil {
		session.cancel()
	}
	select {
	case <-session.done:
	case <-time.After(3 * time.Second):
	}
}

func (a *App) runAgent(ctx context.Context, session *AgentSession) {
	defer close(session.done)
	tabID := session.TabID

	a.safeEmit("agent:event", tabID, AgentEvent{Type: "message_start"})

	var err error
	prov, perr := a.providerFor(session.Provider)
	if perr != nil {
		err = perr
	} else {
		var assistant strings.Builder
		_, err = prov.StreamChat(ctx, session.Model, session.history, func(text string) {
			assistant.WriteString(text)
			a.safeEmit("agent:event", tabID, AgentEvent{Type: "text_delta", Text: text})
		})
		if err == nil {
			session.mu.Lock()
			session.history = append(session.history, ChatTurn{Role: "assistant", Content: assistant.String()})
			session.mu.Unlock()
		}
	}

	switch {
	case err == nil:
		a.safeEmit("agent:event", tabID, AgentEvent{Type: "done"})
		a.safeEmit("session:done", tabID, 0)
	case errors.Is(err, context.Canceled):
		// Superseded by a follow-up or stopped by the user — stay silent; the
		// replacing run (or the frontend stop handler) owns the resulting UI state.
	default:
		a.safeEmit("agent:event", tabID, AgentEvent{Type: "error", Error: err.Error()})
		a.safeEmit("session:done", tabID, 1)
	}
}
