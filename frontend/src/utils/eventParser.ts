import { useTabs } from '../composables/useTabs'

export function parseSessionEvent(tabId: string, rawLine: string) {
  const {
    getTab,
    setTabStatus,
    setTabSessionId,
    addMessage,
    updateLastAssistantMessage,
    setTabCost,
  } = useTabs()

  let outer: any
  try {
    outer = JSON.parse(rawLine)
  } catch {
    return
  }

  const tab = getTab(tabId)
  if (!tab) return

  const type = outer.type

  // system.init — store session ID
  if (type === 'system' && outer.session_id) {
    setTabSessionId(tabId, outer.session_id)
    setTabStatus(tabId, 'thinking')
    return
  }

  // stream_event — unwrap the inner event for streaming
  if (type === 'stream_event' && outer.event) {
    const event = outer.event
    const eventType = event.type

    if (eventType === 'message_start' && event.message?.role === 'assistant') {
      addMessage(tabId, {
        role: 'assistant',
        content: '',
        toolUse: [],
        timestamp: Date.now(),
        isStreaming: true,
      })
      setTabStatus(tabId, 'thinking')
      return
    }

    if (eventType === 'content_block_start' && event.content_block) {
      if (event.content_block.type === 'tool_use') {
        setTabStatus(tabId, 'tool_use')
        updateLastAssistantMessage(tabId, (msg) => {
          msg.toolUse.push({
            id: event.content_block.id || '',
            name: event.content_block.name || '',
            input: '',
            output: undefined,
          })
        })
      }
      return
    }

    if (eventType === 'content_block_delta' && event.delta) {
      if (event.delta.type === 'text_delta' && event.delta.text) {
        updateLastAssistantMessage(tabId, (msg) => {
          msg.content += event.delta.text
        })
      }
      if (event.delta.type === 'input_json_delta' && event.delta.partial_json) {
        updateLastAssistantMessage(tabId, (msg) => {
          const lastTool = msg.toolUse[msg.toolUse.length - 1]
          if (lastTool) {
            lastTool.input += event.delta.partial_json
          }
        })
      }
      return
    }

    if (eventType === 'content_block_stop') {
      return
    }

    if (eventType === 'message_stop') {
      updateLastAssistantMessage(tabId, (msg) => {
        msg.isStreaming = false
      })
      return
    }

    // message_delta (stop reason, usage)
    if (eventType === 'message_delta') {
      return
    }

    return
  }

  // assistant — complete message (also sent alongside stream_events)
  // Skip if we already have a streaming message (avoid duplicates)
  if (type === 'assistant' && outer.message?.content) {
    const msgs = tab.messages
    const lastMsg = msgs.length > 0 ? msgs[msgs.length - 1] : null

    // If the last message is a streaming assistant message, finalize it
    if (lastMsg && lastMsg.role === 'assistant' && lastMsg.isStreaming) {
      const fullText = outer.message.content
        .filter((b: any) => b.type === 'text')
        .map((b: any) => b.text || '')
        .join('')
      lastMsg.content = fullText
      lastMsg.isStreaming = false

      // Add any tool uses not already captured
      const tools = outer.message.content
        .filter((b: any) => b.type === 'tool_use')
      for (const t of tools) {
        const exists = lastMsg.toolUse.some((tu: any) => tu.id === t.id)
        if (!exists) {
          lastMsg.toolUse.push({
            id: t.id || '',
            name: t.name || '',
            input: typeof t.input === 'string' ? t.input : JSON.stringify(t.input || {}),
            output: undefined,
          })
        }
      }
      return
    }

    // No streaming message — add as complete message
    if (!lastMsg || lastMsg.role !== 'assistant' || !lastMsg.isStreaming) {
      const textParts = outer.message.content
        .filter((b: any) => b.type === 'text')
        .map((b: any) => b.text || '')
        .join('')
      const toolParts = outer.message.content
        .filter((b: any) => b.type === 'tool_use')
        .map((b: any) => ({
          id: b.id || '',
          name: b.name || '',
          input: typeof b.input === 'string' ? b.input : JSON.stringify(b.input || {}),
          output: undefined,
        }))

      if (textParts || toolParts.length > 0) {
        addMessage(tabId, {
          role: 'assistant',
          content: textParts,
          toolUse: toolParts,
          timestamp: Date.now(),
          isStreaming: false,
        })
      }
    }
    return
  }

  // user event — tool results and permission denials
  if (type === 'user' && outer.message?.content) {
    const content = outer.message.content
    if (Array.isArray(content)) {
      for (const block of content) {
        if (block.type === 'tool_result') {
          // Match tool result back to tool use and set output
          const toolUseId = block.tool_use_id
          if (toolUseId) {
            // Search all messages for the matching tool
            for (let i = tab.messages.length - 1; i >= 0; i--) {
              const msg = tab.messages[i]
              const tool = msg.toolUse.find(t => t.id === toolUseId)
              if (tool) {
                const resultText = typeof block.content === 'string'
                  ? block.content
                  : Array.isArray(block.content)
                    ? block.content.map((c: any) => c.text || JSON.stringify(c)).join('\n')
                    : JSON.stringify(block.content || '')
                tool.output = resultText || '(no output)'
                break
              }
            }
          }

          // Show permission errors as assistant messages
          if (block.is_error && block.content) {
            const errorText = typeof block.content === 'string'
              ? block.content
              : JSON.stringify(block.content)

            if (errorText.includes('permission')) {
              addMessage(tabId, {
                role: 'assistant',
                content: `**Permission Required:** ${errorText}`,
                toolUse: [],
                timestamp: Date.now(),
                isStreaming: false,
              })
            }
          }
        }
      }
    }
    return
  }

  // result event — session complete
  if (type === 'result') {
    if (outer.session_id) {
      setTabSessionId(tabId, outer.session_id)
    }
    if (outer.total_cost_usd !== undefined) {
      setTabCost(tabId, outer.total_cost_usd)
    }

    // Mark any tools still without output as complete
    for (const msg of tab.messages) {
      for (const tool of msg.toolUse) {
        if (tool.output === undefined) {
          tool.output = '(no output)'
        }
      }
    }

    setTabStatus(tabId, 'done')

    // Revert to idle after 3 seconds
    setTimeout(() => {
      const currentTab = getTab(tabId)
      if (currentTab && currentTab.status === 'done') {
        setTabStatus(tabId, 'idle')
      }
    }, 3000)
    return
  }
}
