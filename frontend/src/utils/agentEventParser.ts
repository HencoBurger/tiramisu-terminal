import { useTabs } from '../composables/useTabs'
import { usePermissions } from '../composables/usePermissions'

// Normalized streaming event from the native chat runtime (Go AgentEvent).
export interface AgentEvent {
  type: string
  text?: string
  error?: string
  toolId?: string
  toolName?: string
  toolInputDelta?: string
  toolOutput?: string
  toolInput?: string
  reqId?: string
}

// Applies an agent:event to the shared tab store — the same mutations parseSessionEvent
// uses, so the existing ChatPanel/MessageBubble/ToolUseBlock renderer is reused.
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

    case 'tool_use_start':
      setTabStatus(tabId, 'tool_use')
      updateLastAssistantMessage(tabId, (msg) => {
        msg.toolUse.push({ id: ev.toolId || '', name: ev.toolName || '', input: '', output: undefined })
      })
      break

    case 'tool_input_delta': {
      const delta = ev.toolInputDelta
      if (delta) {
        updateLastAssistantMessage(tabId, (msg) => {
          const t = msg.toolUse.find((x) => x.id === ev.toolId)
          if (t) t.input += delta
        })
      }
      break
    }

    case 'tool_result':
      // The tool may belong to an earlier assistant message — search backward by id.
      for (let i = tab.messages.length - 1; i >= 0; i--) {
        const t = tab.messages[i].toolUse.find((x) => x.id === ev.toolId)
        if (t) {
          t.output = ev.toolOutput || '(no output)'
          break
        }
      }
      break

    case 'message_stop':
      updateLastAssistantMessage(tabId, (msg) => {
        msg.isStreaming = false
      })
      break

    case 'permission_request':
      usePermissions().set(tabId, {
        reqId: ev.reqId || '',
        toolName: ev.toolName || '',
        toolInput: ev.toolInput || '',
      })
      break

    case 'done':
      updateLastAssistantMessage(tabId, (msg) => {
        msg.isStreaming = false
      })
      setTabStatus(tabId, 'done')
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
