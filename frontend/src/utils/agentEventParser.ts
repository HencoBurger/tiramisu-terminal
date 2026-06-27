import { useTabs } from '../composables/useTabs'

// Normalized streaming event from the native chat runtime (Go AgentEvent).
export interface AgentEvent {
  type: string // message_start | text_delta | done | error
  text?: string
  error?: string
}

// Applies an agent:event to the shared tab store — the same mutations parseSessionEvent
// uses, so the existing ChatPanel/MessageBubble renderer is reused unchanged.
export function parseAgentEvent(tabId: string, ev: AgentEvent) {
  const { getTab, setTabStatus, addMessage, updateLastAssistantMessage } = useTabs()
  const tab = getTab(tabId)
  if (!tab) return

  switch (ev.type) {
    case 'message_start':
      addMessage(tabId, {
        role: 'assistant',
        content: '',
        toolUse: [],
        timestamp: Date.now(),
        isStreaming: true,
      })
      setTabStatus(tabId, 'thinking')
      break

    case 'text_delta': {
      const text = ev.text
      if (text) {
        updateLastAssistantMessage(tabId, (msg) => {
          msg.content += text
        })
      }
      break
    }

    case 'done':
      updateLastAssistantMessage(tabId, (msg) => {
        msg.isStreaming = false
      })
      setTabStatus(tabId, 'done')
      // Revert to idle after 3s (mirrors the Claude 'result' branch).
      setTimeout(() => {
        const t = getTab(tabId)
        if (t && t.status === 'done') setTabStatus(tabId, 'idle')
      }, 3000)
      break

    case 'error':
      updateLastAssistantMessage(tabId, (msg) => {
        msg.isStreaming = false
      })
      addMessage(tabId, {
        role: 'system',
        content: `Error: ${ev.error || 'unknown error'}`,
        toolUse: [],
        timestamp: Date.now(),
        isStreaming: false,
      })
      setTabStatus(tabId, 'error')
      break
  }
}
