<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, defineAsyncComponent } from 'vue'
import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime'
import {
  LoadSessionHistory,
  TerminalStop,
  SetWindowTitle,
  NewWindow,
  CreateWindowSession,
  LoadWindowSession,
} from '../wailsjs/go/main/App'
import { useTabs } from './composables/useTabs'
import { useConfig } from './composables/useConfig'
import { useSound } from './composables/useSound'
import { parseSessionEvent } from './utils/eventParser'
import type { StoredSession } from './types/session'
import TabBar from './components/TabBar.vue'
import ChatPanel from './components/ChatPanel.vue'
import TerminalPanel from './components/TerminalPanel.vue'
// Lazy: Monaco (~5 MB) loads only when an Editor tab is first opened, keeping
// chat/terminal-only startup lean.
const IdePanel = defineAsyncComponent(() => import('./components/ide/IdePanel.vue'))
import SessionBrowser from './components/SessionBrowser.vue'
import SessionPicker from './components/SessionPicker.vue'
import SettingsPanel from './components/SettingsPanel.vue'
import DebugDrawer from './components/DebugDrawer.vue'
import ConfirmDialog from './components/ConfirmDialog.vue'

const {
  tabs,
  activeTabId,
  activeTab,
  addTab,
  setActiveTab,
  setTabStatus,
  setTabSessionId,
  setTabWorkDir,
  renameTab,
  removeTab,
  getTab,
  getTabConfigs,
  restoreTabs,
  addMessage,
} = useTabs()

const {
  effectiveConfig,
  windowSession,
  loadGlobalConfig,
  saveTabState,
  saveWindowName,
  setWindowSession,
} = useConfig()
const { play } = useSound()

const sessionLoaded = ref(false)
const showSessionPicker = ref(false)
const showSessionBrowser = ref(false)
const showSettings = ref(false)
const showDebug = ref(false)
const debugLogs = ref<string[]>([])
const contextMenu = ref<{ tabId: string; x: number; y: number } | null>(null)

// Close-tab confirmation
const pendingClose = ref<{ tabIds: string[]; activateAfter?: string } | null>(null)

const closeMessage = computed(() => {
  if (!pendingClose.value) return ''
  const { tabIds } = pendingClose.value
  if (tabIds.length === 1) {
    const tab = getTab(tabIds[0])
    return `Close tab "${tab?.name || 'Untitled'}"?`
  }
  return `Close ${tabIds.length} tabs?`
})

function requestCloseTabs(tabIds: string[], activateAfter?: string) {
  if (tabIds.length === 0) return
  pendingClose.value = { tabIds, activateAfter }
}

function confirmClose() {
  if (!pendingClose.value) return
  const { tabIds, activateAfter } = pendingClose.value
  for (const id of tabIds) {
    const tab = getTab(id)
    if (tab?.type === 'terminal') TerminalStop(id).catch(() => {})
    removeTab(id)
  }
  if (activateAfter) setActiveTab(activateAfter)
  pendingClose.value = null
}

// Session name inline editing
const isRenamingSession = ref(false)
const sessionNameInput = ref('')
const sessionNameEl = ref<HTMLInputElement>()

function startSessionRename() {
  sessionNameInput.value = windowSession.value?.name || 'Untitled'
  isRenamingSession.value = true
  setTimeout(() => sessionNameEl.value?.select(), 0)
}

function finishSessionRename() {
  const name = sessionNameInput.value.trim()
  saveWindowName(name)
  isRenamingSession.value = false
}

// Debounced tab config save
let saveTimeout: ReturnType<typeof setTimeout> | null = null
function debouncedSaveTabState() {
  if (saveTimeout) clearTimeout(saveTimeout)
  saveTimeout = setTimeout(() => {
    saveTabState(getTabConfigs())
  }, 1000)
}

// Watch tab changes for persistence
watch(
  () => tabs.value.map(t => `${t.id}|${t.name}|${t.workDir}|${t.sessionId}|${t.profileId}|${t.model}|${t.type}|${(t.openFiles ?? []).join('\n')}|${t.activeFile ?? ''}`).join(','),
  () => debouncedSaveTabState(),
)

onMounted(async () => {
  await loadGlobalConfig()
  showSessionPicker.value = true

  // Listen for session events from Go backend
  EventsOn('session:event', (tabId: string, line: string) => {
    debugLogs.value.push(line)
    parseSessionEvent(tabId, line)
  })

  EventsOn('session:done', (tabId: string, exitCode: number) => {
    const tab = getTab(tabId)
    if (!tab) return

    if (exitCode !== 0 && tab.status !== 'done') {
      setTabStatus(tabId, 'error')
    }

    const soundName = tab.soundOverride || effectiveConfig.value.defaultSound
    if (soundName) {
      play(soundName)
    }
  })

  // Keyboard shortcuts
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  EventsOff('session:event')
  EventsOff('session:done')
  document.removeEventListener('keydown', handleKeydown)
})

// Session picker handlers
async function handleSessionCreate(name: string) {
  try {
    const session = await CreateWindowSession(name)
    setWindowSession(session as any)
    addTab()
    sessionLoaded.value = true
  } catch (e) {
    console.error('Failed to create session:', e)
  }
}

async function handleSessionSelect(id: string) {
  try {
    const loaded = await LoadWindowSession(id)
    setWindowSession(loaded as any)

    if (loaded.tabs && loaded.tabs.length > 0) {
      restoreTabs(loaded.tabs as any)
      for (const tabCfg of loaded.tabs) {
        if (tabCfg.sessionId && tabCfg.workDir && (tabCfg.type || 'chat') === 'chat') {
          loadTabHistory(tabCfg.id, tabCfg.sessionId, tabCfg.workDir)
        }
      }
    } else {
      addTab()
    }

    sessionLoaded.value = true
  } catch (e) {
    console.error('Failed to load session:', e)
  }
}

function handleKeydown(e: KeyboardEvent) {
  // Confirm dialog handles its own keys
  if (pendingClose.value) return

  // Close context menu on any key
  if (contextMenu.value) {
    contextMenu.value = null
  }

  if (e.ctrlKey || e.metaKey) {
    switch (e.key) {
      case 't':
        e.preventDefault()
        addTab()
        break
      case 'w':
        e.preventDefault()
        if (activeTab.value) {
          requestCloseTabs([activeTab.value.id])
        }
        break
      case 'd':
        e.preventDefault()
        showDebug.value = !showDebug.value
        break
      case 'Tab':
        e.preventDefault()
        if (tabs.value.length > 1) {
          const idx = tabs.value.findIndex(t => t.id === activeTabId.value)
          const next = e.shiftKey
            ? (idx - 1 + tabs.value.length) % tabs.value.length
            : (idx + 1) % tabs.value.length
          setActiveTab(tabs.value[next].id)
        }
        break
      default:
        // Ctrl+1-9 jump to tab
        const num = parseInt(e.key)
        if (num >= 1 && num <= 9 && num <= tabs.value.length) {
          e.preventDefault()
          setActiveTab(tabs.value[num - 1].id)
        }
        // Ctrl+Shift+S for session browser
        if (e.key === 'S' && e.shiftKey) {
          e.preventDefault()
          showSessionBrowser.value = true
        }
        break
    }
  }
}

function handleTabContextMenu(tabId: string, event: MouseEvent) {
  contextMenu.value = { tabId, x: event.clientX, y: event.clientY }
}

function closeContextMenu() {
  contextMenu.value = null
}

function handleContextMenuAction(action: string) {
  if (!contextMenu.value) return
  const tabId = contextMenu.value.tabId

  switch (action) {
    case 'rename': {
      const name = prompt('Rename tab:', getTab(tabId)?.name)
      if (name) renameTab(tabId, name)
      break
    }
    case 'close': {
      requestCloseTabs([tabId])
      break
    }
    case 'closeOthers': {
      const others = tabs.value.filter(t => t.id !== tabId).map(t => t.id)
      requestCloseTabs(others, tabId)
      break
    }
  }

  contextMenu.value = null
}

async function loadTabHistory(tabId: string, sessionId: string, workDir: string) {
  try {
    const history = await LoadSessionHistory(sessionId, workDir)
    if (!history || history.length === 0) return

    for (const msg of history) {
      addMessage(tabId, {
        role: msg.role === 'user' ? 'user' : 'assistant',
        content: msg.content || '',
        toolUse: (msg.tools || []).map((t: any) => ({
          id: t.id,
          name: t.name,
          input: t.input,
          output: t.output || undefined,
        })),
        timestamp: Date.now(),
        isStreaming: false,
      })
    }
  } catch (e) {
    console.error(`Failed to load history for tab ${tabId}:`, e)
  }
}

function handleGlobalCommand(action: string) {
  switch (action) {
    case 'new':
      addTab()
      break
    case 'sessions':
      showSessionBrowser.value = true
      break
    case 'settings':
      showSettings.value = true
      break
    case 'debug':
      showDebug.value = !showDebug.value
      break
  }
}

function handleResumeSession(session: StoredSession) {
  const tab = addTab(session.projectDir, session.firstPrompt.slice(0, 30) || 'Resumed')
  setTabSessionId(tab.id, session.sessionId)
  setTabWorkDir(tab.id, session.projectDir)
}
</script>

<template>
  <div class="h-screen flex flex-col bg-base-100" @click="closeContextMenu">
    <!-- Session picker (shown on startup, mandatory) -->
    <SessionPicker
      :open="showSessionPicker && !sessionLoaded"
      :mandatory="!sessionLoaded"
      @create="handleSessionCreate"
      @select="handleSessionSelect"
      @update:open="showSessionPicker = $event"
    />

    <!-- Main content (only after session is loaded) -->
    <template v-if="sessionLoaded">
      <!-- Tab bar with session name -->
      <div class="flex items-end bg-base-300 pt-1" style="--wails-draggable: drag">
        <div class="flex items-center px-2 pb-1.5 shrink-0" style="--wails-draggable: no-drag">
          <input
            v-if="isRenamingSession"
            ref="sessionNameEl"
            v-model="sessionNameInput"
            class="input input-ghost input-sm w-32 px-1 text-base-content font-semibold text-sm"
            @blur="finishSessionRename"
            @keydown.enter="finishSessionRename"
            @keydown.escape="isRenamingSession = false"
          />
          <span
            v-else
            class="text-sm font-semibold text-base-content/70 hover:text-base-content cursor-pointer"
            @click="startSessionRename"
            :title="'Click to rename session'"
          >{{ windowSession?.name || 'Untitled' }}</span>
        </div>
        <TabBar class="flex-1" @tab-context-menu="handleTabContextMenu" @tab-close="(id: string) => requestCloseTabs([id])" />
      </div>

      <!-- Main content area: chat + debug side by side -->
      <div class="flex-1 overflow-hidden flex">
        <!-- Chat panels -->
        <div class="flex-1 overflow-hidden relative">
          <div
            v-for="tab in tabs"
            :key="tab.id"
            v-show="tab.id === activeTabId"
            class="absolute inset-0"
          >
            <ChatPanel v-if="tab.type === 'chat' || !tab.type" :tab="tab" @command="handleGlobalCommand" />
            <TerminalPanel v-else-if="tab.type === 'terminal'" :tab="tab" />
            <IdePanel v-else-if="tab.type === 'ide'" :tab="tab" />
          </div>

          <!-- Empty state -->
          <div v-if="tabs.length === 0" class="flex items-center justify-center h-full text-base-content/30">
            <div class="text-center">
              <p class="text-xl mb-4">Welcome to Tiramisu</p>
              <p class="mb-4">Press <kbd class="kbd kbd-sm">Ctrl+T</kbd> or click + to create a new tab</p>
              <div class="flex gap-2 justify-center">
                <button class="btn btn-outline btn-sm" @click="addTab()">New Tab</button>
                <button class="btn btn-outline btn-sm" @click="showSessionBrowser = true">Browse Sessions</button>
              </div>
            </div>
          </div>
        </div>

        <!-- Debug drawer (inline, pushes chat aside) -->
        <DebugDrawer
          v-if="showDebug"
          :open="showDebug"
          :logs="debugLogs"
          @update:open="showDebug = $event"
          @clear="debugLogs = []"
        />
      </div>

      <!-- Status bar -->
      <div class="flex items-center px-3 py-1 text-xs bg-base-300 text-base-content/50 gap-4">
        <span>{{ tabs.length }} tab{{ tabs.length !== 1 ? 's' : '' }}</span>
        <span v-if="activeTab?.workDir" class="font-mono truncate max-w-xs" :title="activeTab.workDir">
          {{ activeTab.workDir }}
        </span>
        <span class="ml-auto flex items-center gap-4">
          <span v-if="activeTab?.totalCost">
            ${{ activeTab.totalCost.toFixed(4) }}
          </span>
          <button class="btn btn-ghost btn-xs" @click="NewWindow().catch(() => {})">New Window</button>
          <button class="btn btn-ghost btn-xs" @click="showSessionPicker = true">Sessions</button>
          <button class="btn btn-ghost btn-xs" @click="showSessionBrowser = true">Browse Claude</button>
          <button class="btn btn-ghost btn-xs" @click="showSettings = true">Settings</button>
          <button class="btn btn-ghost btn-xs" :class="{ 'text-warning': showDebug }" @click="showDebug = !showDebug">Debug</button>
        </span>
      </div>
    </template>

    <!-- Context menu -->
    <div
      v-if="contextMenu"
      class="fixed z-50"
      :style="{ left: contextMenu.x + 'px', top: contextMenu.y + 'px' }"
    >
      <ul class="menu bg-base-200 rounded-box shadow-lg w-48 p-1">
        <li><a @click="handleContextMenuAction('rename')">Rename</a></li>
        <li><a @click="handleContextMenuAction('close')">Close</a></li>
        <li><a @click="handleContextMenuAction('closeOthers')">Close Others</a></li>
      </ul>
    </div>

    <!-- Modals -->
    <SessionPicker
      v-if="sessionLoaded"
      :open="showSessionPicker && sessionLoaded"
      :mandatory="false"
      @create="handleSessionCreate"
      @select="handleSessionSelect"
      @update:open="showSessionPicker = $event"
    />
    <SessionBrowser
      :open="showSessionBrowser"
      @update:open="showSessionBrowser = $event"
      @resume="handleResumeSession"
    />
    <SettingsPanel
      :open="showSettings"
      @update:open="showSettings = $event"
    />
    <ConfirmDialog
      :open="!!pendingClose"
      title="Close tab"
      :message="closeMessage"
      confirm-label="Close"
      danger
      @confirm="confirmClose"
      @cancel="pendingClose = null"
    />
  </div>
</template>
