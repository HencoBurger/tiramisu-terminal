# Plan: Provider-agnostic chat for tiramisu (native agent runtime)

> Status: M0 designed and approved; not yet implemented. M1–M3 sketched.

## Context

Today tiramisu's chat is 100% coupled to the `claude` CLI (`session.go` spawns
`claude -p --output-format stream-json`, streams its NDJSON, parsed by
`eventParser.ts`). The goal is **provider-agnostic** chat — choose the provider/where
the model lives (Claude CLI, local **Ollama**, **OpenRouter**), pick a model from that
provider, and eventually run a full **agent** (tools, file edits, sub-agents, an
orchestrator model + a cheaper "agents/quick-task" model like Claude Code's Haiku
delegation).

**Locked decisions:**
- Build our **own** native agent runtime (not wrap another CLI).
- Keep the existing **Claude CLI backend exactly as-is** (one selectable backend).
- Target providers: **Ollama + OpenRouter** (+ keep Claude CLI).
- Ship in milestones. **This doc = M0** (the first shippable slice); M1–M3 sketched
  so M0's seams are future-proof.

**M0 scope:** plain provider-agnostic **chat** (streaming text, **no tools yet**) for
Ollama + OpenRouter, end-to-end, behind a clean backend abstraction. User picks
provider + model and chats; responses stream into the existing chat UI; completion
fires the existing sound + desktop notification. Everything additive; Claude path
untouched.

## Architecture & seams (established in M0, extended by M1–M3)

```
ChatPanel.handleSend  --branch on tab.provider-->  Claude path (unchanged)
                                                     |  or  native: AgentStart/AgentSend (Go)
Go runAgent goroutine --> Provider.StreamChat (SSE) --> emit `agent:event` deltas
                                                     --> emit `session:done` (reuse sound+Notify)
App.vue EventsOn('agent:event') --> parseAgentEvent.ts --> useTabs store mutations
                                                     --> ChatPanel/MessageBubble render (unchanged)
```

Four seams M0 creates that M1–M3 extend: the **`agent:event`** channel, the
**`parseAgentEvent`** reducer, the **`AgentSession`/`agentSessions`** map (mirrors
`sessions`/`sessMu`), and the **`Provider`** interface.

**Event-contract decision:** a NEW typed `agent:event` channel + small reducer (NOT
masquerading as Claude stream-json). Reason: `eventParser.ts` is hard-coupled to
Anthropic's private wire shape; OpenAI-compatible streaming (and M1 tool_calls / M2
sub-agents) don't fit it. We own a small vocabulary instead. Reuse the existing
`session:done` channel for completion so the sound + `Notify()` wiring works for free.

M0 `agent:event` types (Go struct emitted via `runtime.EventsEmit`, arrives in JS as a
parsed object — no JSON.parse): `{ type, text?, error? }` where type ∈
`message_start | text_delta | done | error`. (M1+ adds tool/agent fields — additive.)

## M0 — Go backend (new files + additive edits)

**New `agent.go`** (mirrors `session.go`'s runtime shape):
- `ChatTurn{Role, Content}`; `AgentSession{TabID, Provider, Model, WorkDir, cancel,
  done, mu, history []ChatTurn}`.
- Bound methods: `AgentStart(tabID, provider, model, workDir, prompt)`,
  `AgentSend(tabID, provider, model, workDir, prompt)`, `AgentStop(tabID)`.
  - `AgentStart`: stop any existing session for the tab, seed `history` with a system
    prompt + the user prompt, launch `runAgent` goroutine, return immediately.
  - `AgentSend`: append user turn + re-run; **self-heals** to a fresh start if the tab
    isn't in the map (after restart or a provider switch) — never hard-errors.
  - `AgentStop`: delete from map, `cancel()`, wait on `done` with a 3s timeout (mirrors
    `stopSession`).
- `runAgent` goroutine: emit `message_start`; resolve provider; `StreamChat` calling
  back `onDelta` → emit `text_delta`; append assistant turn; emit `done` (or `error`);
  always emit `session:done` (0 ok / 1 error). Reuse `safeEmit` (already `closing`-gated).
  Ctx-cancel from `AgentStop` aborts the in-flight HTTP read → `context.Canceled` treated
  as a clean stop.

**New `provider.go`**: `ModelInfo{ID,Name}`; `Provider` interface
(`StreamChat(ctx, model, messages, onDelta)` + `ListModels(ctx)`); `providerFor(name)`
resolving Ollama (base URL from config, default `http://localhost:11434`, no key) vs
OpenRouter (`https://openrouter.ai/api/v1`, key from secrets — error if unset); bound
`ListProviderModels(provider)` with a 10s timeout.

**New `openai_compat.go`** — ONE client for both providers (stdlib `net/http` +
`encoding/json`, **no new deps**). Differs only by base URL + auth header + listing URL.
- Streaming: `POST {base}/chat/completions` with
  `{model, messages, stream:true}`, `Authorization: Bearer <key>` when key set. Read SSE
  with a `bufio.Scanner` (1MB buffer like `session.go:167`): strip `data: `, stop on
  `[DONE]`, skip non-`data:` keep-alive lines, unmarshal `choices[].delta.content` →
  `onDelta`, capture `finish_reason`, surface `error.message`.
- Listing: Ollama `GET {base}/api/tags` → `models[].name`; OpenRouter
  `GET /v1/models` → `data[].{id,name}`.

**New `secrets.go`** — keeps the OpenRouter key OFF the webview (GlobalConfig is read
wholesale by the frontend and persisted 0644):
- `~/.tiramisu/secrets.json` written `0600`; `resolveProviderKey(provider)` (UNEXPORTED,
  not bound) used by `providerFor`.
- Narrow bound methods (raw key never returned): `SetProviderKey(provider, key)`,
  `HasProviderKey(provider) bool`, `DeleteProviderKey(provider)`.

**Edit `app.go`** (additive only): `App` struct +`agentSessions map[string]*AgentSession`
& `agentMu sync.RWMutex`; `NewApp` init; `shutdown` teardown block mirroring the
`sessions` loop; `GlobalConfig` += `OllamaBaseURL string`, `EnabledProviders []string`,
`DefaultModels map[string]string`; `loadGlobalConfig` default OllamaBaseURL.

**Untouched:** `main.go` (App already bound; new methods auto-bind), `session.go`,
`sessionstore.go`, `ide.go`, `notify.go`, `sound.go`, `terminal.go`.

## M0 — Frontend (new files + additive edits)

- **`types/session.ts`**: `TabState.provider: string` + `TabConfig.provider: string`
  (`''`/`'claude'` ⇒ Claude path); `GlobalConfig` += `ollamaBaseURL`, `enabledProviders`,
  `defaultModels`; `ModelInfo {id,name}` (or reuse generated `main.ModelInfo`).
- **`composables/useTabs.ts`**: init `provider:''`, restore `cfg.provider||''`, include in
  `getTabConfigs`, add `setTabProvider` setter (mirror `setTabModel`).
- **`composables/useConfig.ts`**: add the 3 new GlobalConfig fields to default/load/save
  (SaveGlobalConfig replaces the whole struct — must round-trip them).
- **New `composables/useAgent.ts`** (mirror `useSession.ts`): wrap `AgentStart/AgentSend/
  AgentStop/ListProviderModels/SetProviderKey/HasProviderKey/DeleteProviderKey`.
- **New `utils/agentEventParser.ts`**: `parseAgentEvent(tabId, ev)` (~40 lines) using the
  same `useTabs` mutations as `parseSessionEvent` (`addMessage`,
  `updateLastAssistantMessage`, `setTabStatus`). `toolUse` stays `[]` in M0.
- **`App.vue`**: `EventsOn('agent:event', …→parseAgentEvent)` + `EventsOff`; guard the
  Claude-only history load with `(tabCfg.provider||'claude')==='claude'`; add
  `${t.provider}` to the persistence-watch key. The existing `session:done` handler is
  unchanged and now serves both runtimes (sound+Notify for native free). (Optional: title
  the notification by provider.)
- **`components/ChatPanel.vue`**: in `handleSend`, branch `const provider =
  tab.provider||'claude'` — claude ⇒ existing start/send/resume; native ⇒ `agentStart`
  (no prior assistant msg) / `agentSend` (else), `setTabStatus('thinking')` for instant
  feedback, try/catch → error status. Make `/clear` and `/stop` call `AgentStop` for
  native tabs. Mount the picker in the header strip.
- **New `components/ProviderModelPicker.vue`** (header, replaces the model badge / the
  hardcoded `/model` prompt): provider `<select>` from `enabledProviders`; model
  `<select>` — Claude static `[sonnet,opus,haiku,'']`, native populated via
  `ListProviderModels(provider)` with loading/error states. On change: `setTabProvider`
  + `setTabModel`, and for native call `AgentStop(tab.id)` to reset (can't continue a
  Claude/Ollama context across providers; transcript stays on screen, model starts fresh).
- **`components/SettingsPanel.vue`**: a "Providers" section — Ollama base URL input
  (→ `globalConfig.ollamaBaseURL`); OpenRouter key as a masked, write-only input →
  `SetProviderKey`, status via `HasProviderKey`, Clear via `DeleteProviderKey` (never
  populated from backend).
- **`utils/eventParser.ts`** — untouched (Claude path).

**Native new-vs-follow-up:** native tabs have no Claude `sessionId`; the Go
`agentSessions[tabID]` map IS the conversation. Frontend guesses via "any assistant
message present"; `AgentSend` self-heals if wrong. After restart, Go memory is gone →
next send transparently restarts (prior context lost; persistence is M3).

## Critical files

- `agent.go` (new — AgentSession + map + AgentStart/Send/Stop + runAgent goroutine)
- `openai_compat.go` (new — the one SSE client + model listing for both providers)
- `provider.go`, `secrets.go` (new)
- `app.go` (edit — App/NewApp/shutdown/GlobalConfig additions)
- `frontend/src/components/ChatPanel.vue` (edit — handleSend provider branch; mount picker)
- `frontend/src/utils/agentEventParser.ts` (new — reducer bridging to the store)
- `frontend/src/components/ProviderModelPicker.vue` (new), `composables/useAgent.ts` (new)

## Verification

**Build:** `go build ./...` + `go vet ./...`; `cd frontend && npm run build` (strict
`vue-tsc` + vite); `wails build` (confirm new bindings in `wailsjs/go/main/App.d.ts`).

**Manual — Ollama:** `ollama serve` + `ollama pull llama3.1:8b`; Settings base URL
`http://localhost:11434`; new chat → picker → ollama + model → send "hi" → tokens
stream; unfocus → completion sound + desktop banner.

**Manual — OpenRouter:** Settings → paste key → Save (shows "Key set ✓"); picker →
openrouter → pick a (e.g. `:free`) model → send → stream → sound/notify when unfocused.

**Graceful failure:** stop Ollama / bad base URL → error shows as a `system` chat message
+ tab status `error` + "failed" notification, no panic. Empty/bad OpenRouter key →
`AgentStart` returns the "key not set" error (caught) or a 401 surfaces as an `error`
event.

**Claude regression:** a `provider:''`/`'claude'` tab behaves exactly as today; restored
old sessions (no provider field) default to Claude.

## M1–M3 sketch (confirming M0 seams hold)

- **M1 — native tool-use loop + permission gate.** `runAgent` becomes a loop: send a
  `tools` JSON-schema array; on `finish_reason:"tool_calls"`, assemble streamed
  `delta.tool_calls[].function.arguments`, execute, append a `tool` turn, loop. Tools
  reuse `ReadFile`/`WriteFile`/`ListDirectory` (`ide.go`) + a new `bash`
  (`exec.CommandContext` with the `WorkDir` already threaded in M0). New `agent:event`
  types (`tool_use_start/tool_input_delta/tool_use_end/tool_result`) map straight onto the
  existing `ToolUseInfo[]` contract that `ToolUseBlock.vue` renders. **New permission-gate
  UI** (none exists today): a `permission_request` event blocks the Go tool runner on a
  channel; a bound `AgentPermissionDecision(tabID, reqId, approved)` unblocks; the existing
  `permissionMode` config drives auto-approve.
- **M2 — orchestrator + worker (sub-agents).** A `delegate` tool spawns a child
  `AgentSession` (worker model) with its own loop; `agentSessions` becomes a parent→children
  tree; per-tab `OrchestratorModel`/`WorkerModel` from `GlobalConfig.DefaultModels`.
  `AgentEvent` gains `AgentID`/`ParentID`; the reducer renders sub-agent output nested.
- **M3 — polish.** Permission UI polish; IDE **diff review** of agent edits (reuse
  `RunGitDiff` + Monaco `EditorPane`/`MarkdownPreview`, preview `WriteFile` as a diff before
  apply); cost/usage (parse final-chunk `usage` → existing `tab.totalCost` UI); **history
  persistence** as JSONL under `~/.tiramisu/sessions/agent/<id>.jsonl` reusing
  `HistoryMessage`/`HistoryTool` + a `LoadAgentSessionHistory` mirroring `LoadSessionHistory`.

## Out of scope (M0)

Tools / file edits / bash, permission-gate UI, sub-agents + orchestrator/worker split,
cost/usage, native conversation persistence across restart, Anthropic-API-as-native-provider
(Claude stays CLI-only). All are M1–M3.
