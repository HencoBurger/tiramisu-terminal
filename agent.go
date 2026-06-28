package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Native provider-agnostic agent runtime. Mirrors session.go's lifecycle shape (a map
// keyed by tabID + per-session cancel/done), but talks to provider HTTP APIs and runs
// its own tool-use loop with an orchestrator + worker (sub-agent) split. The Claude
// path (session.go) is untouched.

const agentSystemPrompt = `You are a capable coding assistant integrated into the Tiramisu desktop app, working inside the user's project directory.

You have tools: read_file, list_directory, write_file, bash, and delegate.

Read before you act — this is mandatory:
- You do NOT know any file's contents until you have read it with read_file in THIS conversation. Never assume, recall, or invent what a file contains.
- Use list_directory to find the relevant file(s), then read_file each one you will discuss or change BEFORE discussing or changing it. If the user names a file or pastes an error, read that file first.
- If you are about to edit or describe a file you have not read this session, stop and call read_file first.
- Read the files relevant to the task (not the whole project) and understand how they fit together before editing.

When changing code:
- Make the smallest change that solves the task; preserve existing behavior, structure, and style.
- DO NOT make destructive changes — never delete files or large blocks, overwrite unrelated content, or run destructive shell commands (e.g. rm -rf, git reset --hard) unless the user clearly asked for it.
- Prefer editing only the specific lines that need to change.
- write_file targets ONE file: its path must include the filename (e.g. src/app.go), never a directory. To create a file in a folder, append the filename to the folder path.

For well-scoped sub-tasks prefer delegate, which runs a worker agent and returns its result, keeping your own context lean.

Answer in clear Markdown with fenced code blocks. When the task is complete, give a short summary of what you changed.`

const subAgentSystemPrompt = `You are a focused worker agent. Complete the single task you are given using read_file, list_directory, write_file, and bash.

Read before you act: you do not know a file's contents until you read_file it this session — never assume or invent them. Read every file you will change or describe BEFORE doing so. Make the smallest necessary change and never make destructive changes (no deleting files/large blocks or destructive shell commands). Reply with a concise result. Do not ask questions — work autonomously and finish.`

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
	Images     []string   `json:"images,omitempty"`       // data-URL images (user turns)
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
}

type AgentSession struct {
	TabID       string
	Provider    string
	Model       string
	WorkerModel string
	WorkDir     string
	cancel      context.CancelFunc
	done        chan struct{}
	mu          sync.Mutex
	history     []ChatTurn
	readFiles   map[string]bool // files read this run (enforces read-before-write)
}

func (s *AgentSession) markRead(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.readFiles == nil {
		s.readFiles = map[string]bool{}
	}
	s.readFiles[path] = true
}

func (s *AgentSession) hasRead(path string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readFiles[path]
}

// seedReadFiles scans prior history for successful read_file calls so read-tracking
// persists across turns of a conversation (and across app restarts, since history is
// reloaded from disk).
func seedReadFiles(history []ChatTurn, workDir string) map[string]bool {
	read := map[string]bool{}
	pathByID := map[string]string{}
	for _, turn := range history {
		switch turn.Role {
		case "assistant":
			for _, tc := range turn.ToolCalls {
				if tc.Function.Name == "read_file" {
					var args map[string]interface{}
					_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
					if p := argString(args, "path"); p != "" {
						pathByID[tc.ID] = resolvePath(workDir, p)
					}
				}
			}
		case "tool":
			if p, ok := pathByID[turn.ToolCallID]; ok {
				if !strings.HasPrefix(turn.Content, "Error:") && !strings.HasPrefix(turn.Content, "Refused:") {
					read[p] = true
				}
			}
		}
	}
	return read
}

// agentSystem returns the base system prompt plus any user-configured custom
// instructions (the editable "preprompt" from Settings).
func (a *App) agentSystem() string {
	s := agentSystemPrompt
	if ci := strings.TrimSpace(a.globalConfig.CustomInstructions); ci != "" {
		s += "\n\nAdditional user instructions:\n" + ci
	}
	return s
}

// AgentStart begins a fresh native-provider conversation in a tab.
func (a *App) AgentStart(tabID, provider, model, workerModel, workDir, prompt string, images []string) error {
	a.stopAgentForTab(tabID)
	history := []ChatTurn{
		{Role: "system", Content: a.agentSystem()},
		{Role: "user", Content: prompt, Images: images},
	}
	a.startAgentRun(tabID, provider, model, workerModel, workDir, history)
	return nil
}

// AgentSend continues a tab's native conversation. Self-heals to a fresh start if the
// tab isn't tracked (after an app restart or a provider switch) — never hard-errors.
func (a *App) AgentSend(tabID, provider, model, workerModel, workDir, prompt string, images []string) error {
	a.agentMu.RLock()
	session, ok := a.agentSessions[tabID]
	a.agentMu.RUnlock()
	if !ok {
		// Restore persisted context across restarts so follow-ups keep their history.
		if hist, _ := a.loadAgentHistory(tabID); len(hist) > 0 {
			history := append(append([]ChatTurn{}, hist...), ChatTurn{Role: "user", Content: prompt, Images: images})
			a.startAgentRun(tabID, provider, model, workerModel, workDir, history)
			return nil
		}
		return a.AgentStart(tabID, provider, model, workerModel, workDir, prompt, images)
	}

	a.stopAgentForTab(tabID)
	session.mu.Lock()
	history := append(append([]ChatTurn{}, session.history...), ChatTurn{Role: "user", Content: prompt, Images: images})
	session.mu.Unlock()
	a.startAgentRun(tabID, provider, model, workerModel, workDir, history)
	return nil
}

// AgentStop cancels and removes a tab's native conversation.
func (a *App) AgentStop(tabID string) error {
	a.stopAgentForTab(tabID)
	return nil
}

func (a *App) startAgentRun(tabID, provider, model, workerModel, workDir string, history []ChatTurn) {
	ctx, cancel := context.WithCancel(context.Background())
	session := &AgentSession{
		TabID:       tabID,
		Provider:    provider,
		Model:       model,
		WorkerModel: workerModel,
		WorkDir:     workDir,
		cancel:      cancel,
		done:        make(chan struct{}),
		history:     history,
		readFiles:   seedReadFiles(history, workDir),
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
	// Orchestrator tools = the base tools + delegate (sub-agent).
	tools := append(defaultTools(), a.delegateTool(session))
	err := a.agentLoop(ctx, session, session.Provider, session.Model, tools, true)

	tabID := session.TabID
	switch {
	case err == nil:
		a.persistAgentHistory(session)
		a.safeEmit("agent:event", tabID, AgentEvent{Type: "done"})
		a.safeEmit("session:done", tabID, 0)
	case errors.Is(err, context.Canceled):
		// Superseded or stopped — stay silent; the replacing run / stop UI owns state.
	default:
		a.persistAgentHistory(session)
		a.safeEmit("agent:event", tabID, AgentEvent{Type: "error", Error: err.Error()})
		a.safeEmit("session:done", tabID, 1)
	}
}

func (a *App) persistAgentHistory(session *AgentSession) {
	session.mu.Lock()
	hist := append([]ChatTurn{}, session.history...)
	session.mu.Unlock()
	a.saveAgentHistory(session.TabID, hist)
}

// agentLoop drives one model→tools→model cycle until the model stops calling tools.
// When emit is true it streams agent:event updates to the tab; sub-agents run silent.
func (a *App) agentLoop(ctx context.Context, session *AgentSession, provider, model string, tools []Tool, emit bool) error {
	tabID := session.TabID
	prov, err := a.providerFor(provider)
	if err != nil {
		return err
	}

	for iter := 0; iter < maxAgentIterations; iter++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		var cb StreamCallbacks
		emittedTool := map[int]string{}
		if emit {
			a.safeEmit("agent:event", tabID, AgentEvent{Type: "message_start"})
			cb = StreamCallbacks{
				OnText: func(t string) {
					a.safeEmit("agent:event", tabID, AgentEvent{Type: "text_delta", Text: t})
				},
				OnReasoning: func(t string) {
					a.safeEmit("agent:event", tabID, AgentEvent{Type: "reasoning_delta", Text: t})
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
		}

		result, serr := prov.StreamChat(ctx, model, session.history, tools, cb)
		if serr != nil {
			return serr
		}
		if emit {
			a.safeEmit("agent:event", tabID, AgentEvent{Type: "message_stop"})
		}

		for i := range result.ToolCalls {
			if id, ok := emittedTool[i]; ok && id != "" {
				result.ToolCalls[i].ID = id
			} else if result.ToolCalls[i].ID == "" {
				result.ToolCalls[i].ID = uuid.NewString()
			}
			result.ToolCalls[i].Type = "function"
		}

		// Reasoning models put their answer in `reasoning` with empty `content`;
		// keep the conversation coherent by storing reasoning as the turn content
		// when there's nothing else.
		turnContent := result.Content
		if turnContent == "" && len(result.ToolCalls) == 0 && result.Reasoning != "" {
			turnContent = result.Reasoning
		}
		session.mu.Lock()
		session.history = append(session.history, ChatTurn{Role: "assistant", Content: turnContent, ToolCalls: result.ToolCalls})
		session.mu.Unlock()

		if len(result.ToolCalls) == 0 {
			return nil
		}

		for _, tc := range result.ToolCalls {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			out := a.executeTool(ctx, session, tools, tc)
			if emit {
				a.safeEmit("agent:event", tabID, AgentEvent{Type: "tool_result", ToolID: tc.ID, ToolOutput: out})
			}
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

	// Enforce read-before-write: refuse to overwrite an existing file the agent
	// hasn't read this conversation (prevents blind clobbering by weak models).
	if tool.Name == "write_file" {
		path := resolvePath(session.WorkDir, argString(args, "path"))
		if isDir(path) {
			return fmt.Sprintf("Error: %q is a directory, not a file. write_file writes a single file — pass a path that includes the filename (e.g. %s/yourfile.ext).", path, strings.TrimRight(path, "/"))
		}
		if isRegularFile(path) && !session.hasRead(path) {
			return fmt.Sprintf("Refused: you have not read %q yet. Call read_file on it first so you don't overwrite unseen content, then write_file again.", path)
		}
	}

	// Same guard for bash commands that truncate-overwrite an existing file via `>`.
	if tool.Name == "bash" {
		for _, p := range bashOverwriteTargets(argString(args, "command"), session.WorkDir) {
			if !session.hasRead(p) {
				return fmt.Sprintf("Refused: this command overwrites %q (via >), which you have not read yet. Call read_file on it first, then run the command again.", p)
			}
		}
	}

	if !a.checkPermission(ctx, session, tool, tc.Function.Arguments) {
		return "Denied by user."
	}
	out, err := tool.Run(ctx, session.WorkDir, args)

	// Record successful reads so subsequent writes to the same file are allowed.
	if tool.Name == "read_file" && err == nil {
		session.markRead(resolvePath(session.WorkDir, argString(args, "path")))
	}

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

// delegateTool lets the orchestrator hand a self-contained sub-task to a worker agent
// running on the (possibly cheaper) worker model. The sub-agent runs silently and
// returns its final result as the tool output.
func (a *App) delegateTool(session *AgentSession) Tool {
	return Tool{
		Name:        "delegate",
		Description: "Delegate a self-contained sub-task to a worker agent (runs on a separate, possibly cheaper model). Provide a complete task description; the worker runs autonomously with file and bash tools and returns its result. Use for well-scoped research or implementation chunks to keep your own context lean.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task": map[string]interface{}{"type": "string", "description": "A complete, self-contained description of the sub-task."},
			},
			"required": []string{"task"},
		},
		Run: func(ctx context.Context, workDir string, args map[string]interface{}) (string, error) {
			task := argString(args, "task")
			if task == "" {
				return "", fmt.Errorf("delegate requires a 'task'")
			}
			return a.runSubAgent(ctx, session, task)
		},
	}
}

// runSubAgent runs a worker agent to completion on the worker model and returns its
// final text. It uses the base tools (no further delegation) and shares the tab's
// permission gate, so its file/shell mutations still prompt the user.
func (a *App) runSubAgent(ctx context.Context, session *AgentSession, task string) (string, error) {
	model := session.WorkerModel
	if model == "" {
		model = session.Model
	}
	sub := &AgentSession{
		TabID:    session.TabID,
		Provider: session.Provider,
		Model:    model,
		WorkDir:  session.WorkDir,
		history: []ChatTurn{
			{Role: "system", Content: subAgentSystemPrompt},
			{Role: "user", Content: task},
		},
	}
	if err := a.agentLoop(ctx, sub, session.Provider, model, defaultTools(), false); err != nil {
		return "", err
	}
	// The last assistant turn holds the worker's final answer.
	sub.mu.Lock()
	defer sub.mu.Unlock()
	for i := len(sub.history) - 1; i >= 0; i-- {
		if sub.history[i].Role == "assistant" && sub.history[i].Content != "" {
			return sub.history[i].Content, nil
		}
	}
	return "(worker produced no output)", nil
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
		if tool.Name != "bash" {
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
