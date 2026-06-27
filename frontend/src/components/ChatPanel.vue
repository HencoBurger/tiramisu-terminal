<script setup lang="ts">
import { ref, watch, nextTick, computed } from 'vue'
import type { TabState } from '../types/session'
import { useSession } from '../composables/useSession'
import { useTabs } from '../composables/useTabs'
import { useConfig } from '../composables/useConfig'
import { useAgent } from '../composables/useAgent'
import { usePermissions } from '../composables/usePermissions'
import { slashCommands } from '../types/slashCommands'
import { RunGitDiff } from '../../wailsjs/go/main/App'
import MessageBubble from './MessageBubble.vue'
import InlineReply from './InlineReply.vue'
import InputBar from './InputBar.vue'
import WorkDirPicker from './WorkDirPicker.vue'
import ProviderModelPicker from './ProviderModelPicker.vue'

const props = defineProps<{
  tab: TabState
}>()

const { startSession, sendMessage, resumeSession, stopSession } = useSession()
const { addMessage, setTabStatus, setTabWorkDir, setTabProfile, setTabPlanMode, setTabModel, autoNameTab, renameTab } = useTabs()
const { effectiveConfig, maybeSetDefaultWorkDir } = useConfig()
const { agentStart, agentSend, agentStop, agentPermissionDecision, deleteAgentHistory } = useAgent()
const { pending: pendingPerms, clear: clearPerm } = usePermissions()

function permFor() {
  return pendingPerms[props.tab.id]
}
function approvePermission() {
  const req = permFor()
  if (!req) return
  agentPermissionDecision(req.reqId, true).catch(() => {})
  clearPerm(props.tab.id)
}
function denyPermission() {
  const req = permFor()
  if (!req) return
  agentPermissionDecision(req.reqId, false).catch(() => {})
  clearPerm(props.tab.id)
}

const profiles = computed(() => effectiveConfig.value.profiles || [])

const emit = defineEmits<{
  command: [action: string]
}>()

const messagesContainer = ref<HTMLElement>()
const showWorkDirPicker = ref(false)
const error = ref('')
const pendingMessage = ref('')

const needsWorkDir = computed(() => !props.tab.workDir)
const isBusy = computed(() => props.tab.status === 'thinking' || props.tab.status === 'tool_use')

const lastAssistantMessage = computed(() => {
  const msgs = props.tab.messages
  if (msgs.length === 0) return null
  const last = msgs[msgs.length - 1]
  if (last.role === 'assistant' && !last.isStreaming) return last
  return null
})

const showInlineReply = computed(() => {
  if (isBusy.value || needsWorkDir.value) return false
  return lastAssistantMessage.value !== null
})

// Auto-scroll on new messages
watch(
  () => props.tab.messages.length,
  async () => {
    await nextTick()
    scrollToBottom()
  }
)

// Also watch last message content for streaming updates
watch(
  () => {
    const msgs = props.tab.messages
    if (msgs.length === 0) return ''
    return msgs[msgs.length - 1].content
  },
  async () => {
    await nextTick()
    scrollToBottom()
  }
)

function scrollToBottom() {
  if (messagesContainer.value) {
    messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
  }
}

async function handleSend(text: string) {
  if (needsWorkDir.value) {
    showWorkDirPicker.value = true
    return
  }

  error.value = ''

  // Add user message to chat
  addMessage(props.tab.id, {
    role: 'user',
    content: text,
    toolUse: [],
    timestamp: Date.now(),
    isStreaming: false,
  })

  // Auto-name tab on first user message
  const userMessages = props.tab.messages.filter(m => m.role === 'user')
  if (userMessages.length === 1) {
    autoNameTab(props.tab.id, text)
  }

  // Plan mode: prepend planning prefix to the prompt sent to Claude
  let promptToSend = text
  if (props.tab.planMode) {
    promptToSend = `Before taking any action, first create a detailed plan. Outline what files you'll change, what approach you'll take, and any risks. Present the plan and ask for confirmation before proceeding. Do NOT make any edits or tool calls until the plan is approved.\n\nUser's request:\n${text}`
  }

  // Native providers (Ollama/OpenRouter) go through the agent runtime, not Claude CLI.
  const provider = props.tab.provider || 'claude'
  if (provider !== 'claude') {
    setTabStatus(props.tab.id, 'thinking')
    const hasAssistant = props.tab.messages.some(m => m.role === 'assistant')
    try {
      const worker = props.tab.workerModel || ''
      if (hasAssistant) {
        await agentSend(props.tab.id, provider, props.tab.model, worker, props.tab.workDir, promptToSend)
      } else {
        await agentStart(props.tab.id, provider, props.tab.model, worker, props.tab.workDir, promptToSend)
      }
    } catch (e: any) {
      error.value = e?.message || String(e)
      setTabStatus(props.tab.id, 'error')
    }
    return
  }

  const model = props.tab.model

  try {
    if (props.tab.sessionId) {
      try {
        await sendMessage(props.tab.id, promptToSend, props.tab.profileId, model)
      } catch {
        // Go session map lost (e.g. after restart) — resume with stored sessionId
        await resumeSession(props.tab.id, props.tab.workDir, props.tab.sessionId, promptToSend, props.tab.profileId, model)
      }
    } else {
      await startSession(props.tab.id, props.tab.workDir, promptToSend, props.tab.profileId, model)
    }
  } catch (e: any) {
    const msg = e?.message || String(e)
    if (msg.includes('no such file or directory') || msg.includes('does not exist')) {
      error.value = `Directory "${props.tab.workDir}" no longer exists. Please choose a new working directory.`
      pendingMessage.value = text
      showWorkDirPicker.value = true
    } else {
      error.value = msg
    }
    setTabStatus(props.tab.id, 'error')
  }
}

function handleWorkDirSelect(dir: string) {
  setTabWorkDir(props.tab.id, dir)
  maybeSetDefaultWorkDir(dir)
  error.value = ''

  // Retry the pending message with the new directory
  if (pendingMessage.value) {
    const text = pendingMessage.value
    pendingMessage.value = ''
    nextTick(async () => {
      try {
        setTabStatus(props.tab.id, 'thinking')
        if (props.tab.sessionId) {
          await resumeSession(props.tab.id, dir, props.tab.sessionId, text, props.tab.profileId, props.tab.model)
        } else {
          await startSession(props.tab.id, dir, text, props.tab.profileId, props.tab.model)
        }
      } catch (e: any) {
        error.value = e?.message || String(e)
        setTabStatus(props.tab.id, 'error')
      }
    })
  }
}

async function handleCommand(action: string) {
  switch (action) {
    case 'clear':
      props.tab.messages.splice(0)
      if ((props.tab.provider || 'claude') !== 'claude') {
        agentStop(props.tab.id).catch(() => {})
        deleteAgentHistory(props.tab.id).catch(() => {})
      }
      break
    case 'stop':
      if ((props.tab.provider || 'claude') === 'claude') stopSession(props.tab.id).catch(() => {})
      else agentStop(props.tab.id).catch(() => {})
      break
    case 'workdir':
      showWorkDirPicker.value = true
      break
    case 'rename': {
      const name = prompt('Rename tab:', props.tab.name)
      if (name) renameTab(props.tab.id, name)
      break
    }
    case 'plan': {
      const newState = !props.tab.planMode
      setTabPlanMode(props.tab.id, newState)
      addMessage(props.tab.id, {
        role: 'system',
        content: newState
          ? 'Plan mode **enabled**. Claude will outline a plan before taking action.'
          : 'Plan mode **disabled**. Claude will act directly on requests.',
        toolUse: [],
        timestamp: Date.now(),
        isStreaming: false,
      })
      break
    }
    case 'help': {
      const lines = slashCommands.map(c => `- \`${c.name}\` — ${c.description}`)
      addMessage(props.tab.id, {
        role: 'system',
        content: '**Available Commands**\n\n' + lines.join('\n'),
        toolUse: [],
        timestamp: Date.now(),
        isStreaming: false,
      })
      break
    }
    case 'status': {
      const t = props.tab
      const lines = [
        `**Session Status**`,
        `- **Tab:** ${t.name}`,
        `- **Work Dir:** \`${t.workDir || '(none)'}\``,
        `- **Session ID:** \`${t.sessionId || '(none)'}\``,
        `- **Status:** ${t.status}`,
        `- **Messages:** ${t.messages.length}`,
        `- **Cost:** $${t.totalCost.toFixed(4)}`,
        `- **Plan Mode:** ${t.planMode ? 'on' : 'off'}`,
        `- **Model:** ${t.model || '(default)'}`,
      ]
      addMessage(props.tab.id, {
        role: 'system',
        content: lines.join('\n'),
        toolUse: [],
        timestamp: Date.now(),
        isStreaming: false,
      })
      break
    }
    case 'diff': {
      if (!props.tab.workDir) {
        addMessage(props.tab.id, {
          role: 'system',
          content: 'No working directory set. Use `/workdir` first.',
          toolUse: [],
          timestamp: Date.now(),
          isStreaming: false,
        })
        break
      }
      try {
        const diff = await RunGitDiff(props.tab.workDir)
        addMessage(props.tab.id, {
          role: 'system',
          content: diff
            ? '**Git Diff**\n\n```diff\n' + diff + '\n```'
            : 'No changes detected (working tree clean).',
          toolUse: [],
          timestamp: Date.now(),
          isStreaming: false,
        })
      } catch (e: any) {
        addMessage(props.tab.id, {
          role: 'system',
          content: `Git diff failed: ${e?.message || String(e)}`,
          toolUse: [],
          timestamp: Date.now(),
          isStreaming: false,
        })
      }
      break
    }
    case 'model': {
      const choice = prompt('Enter model (sonnet, opus, haiku, or empty for default):')
      if (choice === null) break // cancelled
      const model = choice.trim().toLowerCase()
      const valid = ['sonnet', 'opus', 'haiku', '']
      if (!valid.includes(model)) {
        addMessage(props.tab.id, {
          role: 'system',
          content: `Invalid model "${model}". Choose: sonnet, opus, haiku, or empty for default.`,
          toolUse: [],
          timestamp: Date.now(),
          isStreaming: false,
        })
        break
      }
      setTabModel(props.tab.id, model)
      addMessage(props.tab.id, {
        role: 'system',
        content: model
          ? `Model switched to **${model}**. Next message will use this model.`
          : 'Model reset to **default**.',
        toolUse: [],
        timestamp: Date.now(),
        isStreaming: false,
      })
      break
    }
    case 'compact': {
      // Clear local messages and send a summarization prompt
      props.tab.messages.splice(0)
      const compactPrompt = 'Please provide a brief summary of our conversation so far, highlighting key decisions, changes made, and any pending items.'
      try {
        if (props.tab.sessionId) {
          await resumeSession(props.tab.id, props.tab.workDir, props.tab.sessionId, compactPrompt, props.tab.profileId, props.tab.model)
        } else {
          addMessage(props.tab.id, {
            role: 'system',
            content: 'No active session to compact.',
            toolUse: [],
            timestamp: Date.now(),
            isStreaming: false,
          })
        }
      } catch (e: any) {
        addMessage(props.tab.id, {
          role: 'system',
          content: `Compact failed: ${e?.message || String(e)}`,
          toolUse: [],
          timestamp: Date.now(),
          isStreaming: false,
        })
      }
      break
    }
    default:
      // Bubble up to App.vue for global commands (new, sessions, settings, debug)
      emit('command', action)
      break
  }
}
</script>

<template>
  <div class="flex flex-col h-full">
    <!-- Work dir + profile indicator -->
    <div class="px-3 py-1 text-xs text-base-content/50 bg-base-200 border-b border-base-300 flex items-center gap-2">
      <span v-if="tab.workDir" class="font-mono">{{ tab.workDir }}</span>
      <button
        class="btn btn-xs"
        :class="tab.planMode ? 'btn-warning' : 'btn-ghost'"
        @click="handleCommand('plan')"
      >Plan Mode</button>
      <ProviderModelPicker :tab="tab" />
      <select
        v-if="profiles.length > 0"
        :value="tab.profileId"
        class="select select-ghost select-xs text-xs h-5 min-h-0 pl-1 pr-5"
        @change="setTabProfile(tab.id, ($event.target as HTMLSelectElement).value)"
      >
        <option value="">Default profile</option>
        <option v-for="p in profiles" :key="p.id" :value="p.id">{{ p.name }}</option>
      </select>
      <span v-if="tab.totalCost > 0" class="ml-auto">${{ tab.totalCost.toFixed(4) }}</span>
    </div>

    <!-- Messages -->
    <div ref="messagesContainer" class="flex-1 overflow-y-auto p-4 space-y-4">
      <div v-if="tab.messages.length === 0" class="flex items-center justify-center h-full">
        <div class="text-center text-base-content/30">
          <p class="text-lg mb-2">No messages yet</p>
          <p v-if="needsWorkDir" class="text-sm">
            Select a working directory to get started
            <button class="btn btn-sm btn-outline ml-2" @click="showWorkDirPicker = true">
              Choose Directory
            </button>
          </p>
          <p v-else class="text-sm">Send a message to start a Claude session</p>
        </div>
      </div>

      <MessageBubble v-for="msg in tab.messages" :key="msg.id" :message="msg" />

      <!-- Inline reply for answering Claude's questions -->
      <InlineReply v-if="showInlineReply" :message="lastAssistantMessage!.content" @send="handleSend" />

      <!-- Thinking indicator -->
      <div v-if="isBusy && (tab.messages.length === 0 || !tab.messages[tab.messages.length - 1]?.isStreaming)" class="chat chat-start">
        <div class="chat-bubble">
          <span class="loading loading-dots loading-sm"></span>
        </div>
      </div>
    </div>

    <!-- Permission request (native agent tool gate) -->
    <div v-if="pendingPerms[tab.id]" class="px-3 py-2 bg-warning/15 border-t border-warning/40 text-sm">
      <div class="flex items-center gap-2 mb-1">
        <span class="font-semibold text-warning">Permission required</span>
        <span class="opacity-70">Run tool <code class="px-1 rounded bg-base-300">{{ pendingPerms[tab.id].toolName }}</code>?</span>
      </div>
      <pre class="text-xs bg-base-300/60 rounded p-2 overflow-x-auto max-h-32 mb-2">{{ pendingPerms[tab.id].toolInput }}</pre>
      <div class="flex gap-2">
        <button class="btn btn-success btn-xs" @click="approvePermission">Approve</button>
        <button class="btn btn-ghost btn-xs" @click="denyPermission">Deny</button>
      </div>
    </div>

    <!-- Error toast -->
    <div v-if="error" class="px-3 py-2 bg-error/20 text-error text-sm flex items-center gap-2">
      <span>{{ error }}</span>
      <button class="btn btn-ghost btn-xs" @click="error = ''">dismiss</button>
    </div>

    <!-- Input -->
    <InputBar
      :disabled="isBusy"
      :placeholder="needsWorkDir ? 'Select a working directory first...' : undefined"
      @send="handleSend"
      @command="handleCommand"
    />

    <WorkDirPicker
      :open="showWorkDirPicker"
      @update:open="showWorkDirPicker = $event"
      @select="handleWorkDirSelect"
    />
  </div>
</template>
