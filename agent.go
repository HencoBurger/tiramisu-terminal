package main

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Native provider-agnostic agent runtime. Mirrors session.go's lifecycle shape (a map
// keyed by tabID + per-session cancel/done), but talks to provider HTTP APIs and runs
// its own tool-use loop. The Claude path (session.go) is untouched.

const agentSystemPrompt = `You are a capable coding assistant integrated into the Tiramisu desktop app, working inside the user's project directory.

You have tools: read_file, list_directory, write_file, and bash. Use them to inspect and modify the project as needed — don't guess file contents, read them. When you change files, do it with write_file. Prefer small, verifiable steps.

Answer in clear Markdown with fenced code blocks. When the task is complete, give a short summary of what you did.`

const maxAgentIterations = 24

// ToolCallFunction / ToolCall mirror the OpenAI tool-call wire shape.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ChatTurn is one message in a conversation (OpenAI-compatible shape).
type ChatTurn struct {
	Role       string     `json:"role"` // system | user | assistant | tool
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // assistant turns that call tools
	ToolCallID string     `json:"tool_call_id,omitempty"` // tool-result turns
}

// AgentEvent is the normalized streaming event emitted on the "agent:event" channel.
type AgentEvent struct {
	Type           string `json:"type"`
	Text           string `json:"text,omitempty"`
	Error          string `json:"error,omitempty"`
	ToolID         string `json:"toolId,omitempty"`
	ToolName       string `json:"toolName,omitempty"`
	ToolInputDelta string `json:"toolInputDelta,omitempty"`
	ToolOutput     string `json:"toolOutput,omitempty"`
	ToolInput      string `json:"toolInput,omitempty"`
	ReqID          string `json:"reqId,omitempty"`
	// M2 sub-agents (additive).
	AgentID  string `json:"agentId,omitempty"`
	ParentID string `json:"parentId,omitempty"`
}

type AgentSession struct {
	TabID    string
	Provider string
	Model    string
	WorkDir  string
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
	err := a.agentLoop(ctx, session, session.Provider, session.Model, defaultTools())

	tabID := session.TabID
	switch {
	case err == nil:
		a.safeEmit("agent:event", tabID, AgentEvent{Type: "done"})
		a.safeEmit("session:done", tabID, 0)
	case errors.Is(err, context.Canceled):
		// Superseded or stopped — stay silent; the replacing run / stop UI owns state.
	default:
		a.safeEmit("agent:event", tabID, AgentEvent{Type: "error", Error: err.Error()})
		a.safeEmit("session:done", tabID, 1)
	}
}

// agentLoop drives one model→tools→model cycle until the model stops calling tools.
// Returns nil on a clean finish, or an error. Appends turns to session.history.
func (a *App) agentLoop(ctx context.Context, session *AgentSession, provider, model string, tools []Tool) error {
	tabID := session.TabID
	prov, err := a.providerFor(provider)
	if err != nil {
		return err
	}

	for iter := 0; iter < maxAgentIterations; iter++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		a.safeEmit("agent:event", tabID, AgentEvent{Type: "message_start"})

		emittedTool := map[int]string{} // delta index -> emitted toolId
		cb := StreamCallbacks{
			OnText: func(t string) {
				a.safeEmit("agent:event", tabID, AgentEvent{Type: "text_delta", Text: t})
			},
			OnToolStart: func(index int, id, name string) {
				if id == "" {
					id = uuid.NewString()
				}
				emittedTool[index] = id
				a.safeEmit("agent:event", tabID, AgentEvent{Type: "tool_use_start", ToolID: id, ToolName: name})
			},
			OnToolArgs: func(index int, delta string) {
				a.safeEmit("agent:event", tabID, AgentEvent{Type: "tool_input_delta", ToolID: emittedTool[index], ToolInputDelta: delta})
			},
		}

		result, serr := prov.StreamChat(ctx, model, session.history, tools, cb)
		if serr != nil {
			return serr
		}
		a.safeEmit("agent:event", tabID, AgentEvent{Type: "message_stop"})

		// Normalize tool-call IDs to the ones we surfaced to the UI.
		for i := range result.ToolCalls {
			if id, ok := emittedTool[i]; ok && id != "" {
				result.ToolCalls[i].ID = id
			} else if result.ToolCalls[i].ID == "" {
				result.ToolCalls[i].ID = uuid.NewString()
			}
			result.ToolCalls[i].Type = "function"
		}

		session.mu.Lock()
		session.history = append(session.history, ChatTurn{Role: "assistant", Content: result.Content, ToolCalls: result.ToolCalls})
		session.mu.Unlock()

		if len(result.ToolCalls) == 0 {
			return nil // model produced a final answer
		}

		for _, tc := range result.ToolCalls {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			out := a.executeTool(ctx, session, tools, tc)
			a.safeEmit("agent:event", tabID, AgentEvent{Type: "tool_result", ToolID: tc.ID, ToolOutput: out})
			session.mu.Lock()
			session.history = append(session.history, ChatTurn{Role: "tool", Content: out, ToolCallID: tc.ID})
			session.mu.Unlock()
		}
	}
	return nil
}

func (a *App) executeTool(ctx context.Context, session *AgentSession, tools []Tool, tc ToolCall) string {
	tool := findTool(tools, tc.Function.Name)
	if tool == nil {
		return "Error: unknown tool " + tc.Function.Name
	}
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		args = map[string]interface{}{}
	}
	if !a.checkPermission(ctx, session, tool, tc.Function.Arguments) {
		return "Denied by user."
	}
	out, err := tool.Run(ctx, session.WorkDir, args)
	if err != nil {
		if out != "" {
			return out + "\nError: " + err.Error()
		}
		return "Error: " + err.Error()
	}
	if out == "" {
		return "(no output)"
	}
	return out
}

// checkPermission returns whether a tool call may proceed, blocking for an interactive
// decision when the permission mode requires it.
func (a *App) checkPermission(ctx context.Context, session *AgentSession, tool *Tool, argsJSON string) bool {
	if !tool.Mutating {
		return true
	}
	switch a.effectivePermissionMode() {
	case "bypassPermissions":
		return true
	case "acceptEdits":
		if tool.Name != "bash" { // auto-approve file edits, still gate shell commands
			return true
		}
	}

	reqID := uuid.NewString()
	ch := make(chan bool, 1)
	a.permMu.Lock()
	a.pendingPerms[reqID] = ch
	a.permMu.Unlock()

	a.safeEmit("agent:event", session.TabID, AgentEvent{
		Type: "permission_request", ReqID: reqID, ToolName: tool.Name, ToolInput: argsJSON,
	})

	select {
	case ok := <-ch:
		return ok
	case <-ctx.Done():
		a.permMu.Lock()
		delete(a.pendingPerms, reqID)
		a.permMu.Unlock()
		return false
	}
}

// AgentPermissionDecision resolves a pending permission request (bound to the frontend).
func (a *App) AgentPermissionDecision(reqID string, approved bool) {
	a.permMu.Lock()
	ch, ok := a.pendingPerms[reqID]
	if ok {
		delete(a.pendingPerms, reqID)
	}
	a.permMu.Unlock()
	if ok {
		ch <- approved
	}
}
