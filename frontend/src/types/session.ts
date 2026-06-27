export type SessionStatus = 'idle' | 'thinking' | 'tool_use' | 'done' | 'error'
export type TabType = 'chat' | 'terminal' | 'ide'

export interface ToolUseInfo {
  id: string
  name: string
  input: string
  output?: string
}

export interface ChatMessage {
  id: string
  role: 'user' | 'assistant' | 'system'
  content: string
  toolUse: ToolUseInfo[]
  timestamp: number
  isStreaming: boolean
  reasoning?: string // reasoning-model "thinking" output (native runtime)
}

export interface TabState {
  id: string
  name: string
  workDir: string
  sessionId: string
  status: SessionStatus
  messages: ChatMessage[]
  totalCost: number
  soundOverride: string
  profileId: string
  planMode: boolean
  model: string
  // Chat backend provider: '' / 'claude' = Claude CLI; 'ollama' / 'openrouter' = native runtime.
  provider: string
  // Native runtime: worker/sub-agent model used by the delegate tool (orchestrator = model).
  workerModel?: string
  type: TabType
  activity: boolean
  // IDE tabs only: open file paths + active file, for restore across restarts.
  openFiles?: string[]
  activeFile?: string
  // Terminal tabs: a command to run once when the PTY starts. Transient — never
  // persisted to TabConfig, so a "run" never replays on restart.
  initialCommand?: string
}

export interface ClaudeStreamEvent {
  type: string
  subtype?: string
  session_id?: string
  // system.init
  // assistant message events
  message?: {
    id: string
    role: string
    content: ContentBlock[]
    model?: string
    usage?: { input_tokens: number; output_tokens: number }
  }
  // content_block_start / content_block_delta / content_block_stop
  index?: number
  content_block?: ContentBlock
  delta?: {
    type: string
    text?: string
    partial_json?: string
  }
  // result event
  result?: {
    session_id: string
    cost_usd?: number
    duration_ms?: number
    total_cost_usd?: number
  }
}

export interface ContentBlock {
  type: string
  text?: string
  id?: string
  name?: string
  input?: any
}

export interface StoredSession {
  sessionId: string
  projectDir: string
  firstPrompt: string
  messageCount: number
  lastModified: number
}

export interface Profile {
  id: string
  name: string
  homeDir: string
}

export interface TabConfig {
  id: string
  name: string
  workDir: string
  sessionId: string
  soundOverride: string
  profileId: string
  model: string
  provider: string
  workerModel?: string
  type: TabType
  openFiles?: string[]
  activeFile?: string
}

export interface AppConfig {
  defaultSound: string
  theme: string
  permissionMode: string
  projectName?: string
  tabs: TabConfig[]
  profiles: Profile[]
}

export interface GlobalConfig {
  defaultSound: string
  theme: string
  permissionMode: string
  profiles: Profile[]
  ollamaBaseURL: string
  enabledProviders: string[]
  defaultModels: Record<string, string>
}

export interface WindowSession {
  id: string
  name: string
  tabs: TabConfig[]
  defaultWorkDir?: string
  themeOverride?: string
  soundOverride?: string
  permModeOverride?: string
  createdAt: number
  lastOpenedAt: number
}

export interface WindowSessionSummary {
  id: string
  name: string
  tabCount: number
  lastOpenedAt: number
}

export interface EffectiveConfig {
  theme: string
  defaultSound: string
  permissionMode: string
  profiles: Profile[]
}
