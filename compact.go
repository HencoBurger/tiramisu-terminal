package main

import (
	"context"
	"fmt"
	"strings"
)

const defaultContextBudgetTokens = 6000

// estTokens roughly estimates a history's token footprint: text ≈ chars/4, images
// counted flat (they tokenize to a fixed-ish cost regardless of base64 length).
func estTokens(history []ChatTurn) int {
	chars := 0
	images := 0
	for _, t := range history {
		chars += len(t.Content)
		for _, tc := range t.ToolCalls {
			chars += len(tc.Function.Name) + len(tc.Function.Arguments)
		}
		images += len(t.Images)
	}
	return chars/4 + images*1200
}

func clip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…[truncated]"
}

// renderForSummary turns turns into bounded plain text (images dropped, big content
// truncated) so summarizing them doesn't itself overflow the worker's context.
func renderForSummary(turns []ChatTurn) string {
	var b strings.Builder
	for _, t := range turns {
		content := clip(t.Content, 1500)
		if len(t.Images) > 0 {
			content += fmt.Sprintf(" [%d image(s) omitted]", len(t.Images))
		}
		for _, tc := range t.ToolCalls {
			content += fmt.Sprintf(" [called %s(%s)]", tc.Function.Name, clip(tc.Function.Arguments, 200))
		}
		if strings.TrimSpace(content) == "" {
			continue
		}
		b.WriteString(t.Role)
		b.WriteString(": ")
		b.WriteString(content)
		b.WriteString("\n")
	}
	return b.String()
}

const compactSummaryPrompt = `Summarize the following conversation so another agent can continue the task with no loss of essential information. Capture: the user's goal/task, files inspected and key facts/contents about them, decisions and changes made so far, and the current state / next step. Be concise but keep concrete technical details (file names, function names, specifics) needed to continue. Output only the summary.

CONVERSATION:
`

// maybeCompact summarizes older turns when the history exceeds the context budget,
// replacing them with one summary while keeping the system prompt and the latest user
// exchange verbatim. Best-effort: on any failure it leaves history unchanged.
func (a *App) maybeCompact(ctx context.Context, session *AgentSession) {
	budget := a.globalConfig.ContextBudgetTokens
	if budget <= 0 {
		budget = defaultContextBudgetTokens
	}

	session.mu.Lock()
	history := append([]ChatTurn{}, session.history...)
	session.mu.Unlock()

	if estTokens(history) <= budget || len(history) < 3 {
		return
	}

	// Clean boundary: summarize everything between the system prompt and the last
	// user turn; keep the system prompt, the summary, and the last user exchange.
	lastUser := -1
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "user" {
			lastUser = i
			break
		}
	}
	if lastUser <= 1 {
		return // nothing older than the current turn to summarize (single big turn)
	}

	systemTurn := history[0]
	toSummarize := history[1:lastUser]
	keep := history[lastUser:]

	prov, err := a.providerFor(session.Provider)
	if err != nil {
		return
	}
	model := session.WorkerModel
	if model == "" {
		model = session.Model
	}

	a.safeEmit("agent:event", session.TabID, AgentEvent{Type: "notice", Text: "Context window filling up — summarizing earlier conversation…"})

	rendered := clip(renderForSummary(toSummarize), budget*4) // keep the summary input bounded
	res, serr := prov.StreamChat(ctx, model, []ChatTurn{{Role: "user", Content: compactSummaryPrompt + rendered}}, nil, StreamCallbacks{})
	if serr != nil {
		return
	}
	summary := res.Content
	if strings.TrimSpace(summary) == "" {
		summary = res.Reasoning
	}
	if strings.TrimSpace(summary) == "" {
		return
	}

	newHistory := []ChatTurn{
		systemTurn,
		{Role: "user", Content: "[Summary of earlier conversation]\n" + strings.TrimSpace(summary)},
	}
	newHistory = append(newHistory, keep...)

	session.mu.Lock()
	session.history = newHistory
	session.mu.Unlock()
	a.persistAgentHistory(session)

	a.safeEmit("agent:event", session.TabID, AgentEvent{Type: "notice", Text: "Earlier conversation summarized to free up context."})
}
