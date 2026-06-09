<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime'
import { LoadSessionHistory, TerminalStop, SetWindowTitle, NewWindow } from '../wailsjs/go/main/App'
import { useTabs } from './composables/useTabs'
import { useConfig } from './composables/useConfig'
import { useSound } from './composables/useSound'
import { parseSessionEvent } from './utils/eventParser'
import type { StoredSession } from './types/session'
import TabBar from './components/TabBar.vue'
import ChatPanel from './components/ChatPanel.vue'
import TerminalPanel from './components/TerminalPanel.vue'
import SessionBrowser from './components/SessionBrowser.vue'
import SettingsPanel from './components/SettingsPanel.vue'
import DebugDrawer from './components/DebugDrawer.vue'

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

const { config, projectName, loadConfig, saveTabState, saveProjectName } = useConfig()
const { play } = useSound()

const showSessionBrowser = ref(false)
const showSettings = ref(false)
const showDebug = ref(false)
const debugLogs = ref<string[]>([])
const contextMenu = ref<{ tabId: string; x: number; y: number } | null>(null)

// Project name inline editing
const isRenamingProject = ref(false)
const projectNameInput = ref('')
const projectNameEl = ref<HTMLInputElement>()

function startProjectRename() {
  projectNameInput.value = projectName.value
  isRenamingProject.value = true
  setTimeout(() => projectNameEl.value?.select(), 0)
}

function finishProjectRename() {
  const name = projectNameInput.value.trim()
  saveProjectName(name)
  isRenamingProject.value = false
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
  () => tabs.value.map(t => `${t.id}|${t.name}|${t.workDir}|${t.sessionId}|${t.profileId}|${t.model}|${t.type}`).join(','),
  () => debouncedSaveTabState(),
)

// Set window title from project name on startup (handled in onMounted via loadConfig -> SetProjectName)
// No tab-based title updates needed — window title is purely the project name

onMounted(async () => {
  await loadConfig()
  SetWindowTitle(projectName.value).catch(() => {})

  // Restore tabs from config
  if (config.value.tabs && config.value.tabs.length > 0) {
    restoreTabs(config.value.tabs)

    // Load conversation history for chat tabs with existing sessions
    for (const tabCfg of config.value.tabs) {
      if (tabCfg.sessionId && tabCfg.workDir && (tabCfg.type || 'chat') === 'chat') {
        loadTabHistory(tabCfg.id, tabCfg.sessionId, tabCfg.workDir)
      }
    }
  } else {
    addTab()
  }

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

    // Play sound when session completes
    const soundName = tab.soundOverride || config.value.defaultSound
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

function handleKeydown(e: KeyboardEvent) {
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
          if (activeTab.value.type === 'terminal') {
            TerminalStop(activeTab.value.id).catch(() => {})
          }
          removeTab(activeTab.value.id)
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
      const tab = getTab(tabId)
      if (tab?.type === 'terminal') TerminalStop(tabId).catch(() => {})
      removeTab(tabId)
      break
    }
    case 'closeOthers': {
      const others = tabs.value.filter(t => t.id !== tabId)
      others.forEach(t => {
        if (t.type === 'terminal') TerminalStop(t.id).catch(() => {})
        removeTab(t.id)
      })
      setActiveTab(tabId)
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
    <!-- Tab bar with project title -->
    <div class="flex items-end bg-base-300 pt-1" style="--wails-draggable: drag">
      <div class="flex items-center px-2 pb-1.5 shrink-0" style="--wails-draggable: no-drag">
        <input
          v-if="isRenamingProject"
          ref="projectNameEl"
          v-model="projectNameInput"
          class="input input-ghost input-sm w-32 px-1 text-base-content font-semibold text-sm"
          @blur="finishProjectRename"
          @keydown.enter="finishProjectRename"
          @keydown.escape="isRenamingProject = false"
        />
        <span
          v-else
          class="text-sm font-semibold text-base-content/70 hover:text-base-content cursor-pointer"
          @click="startProjectRename"
          :title="'Click to rename project'"
        >{{ projectName }}</span>
      </div>
      <TabBar class="flex-1" @tab-context-menu="handleTabContextMenu" />
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
        <button class="btn btn-ghost btn-xs" @click="showSessionBrowser = true">Sessions</button>
        <button class="btn btn-ghost btn-xs" @click="showSettings = true">Settings</button>
        <button class="btn btn-ghost btn-xs" :class="{ 'text-warning': showDebug }" @click="showDebug = !showDebug">Debug</button>
      </span>
    </div>

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
    <SessionBrowser
      :open="showSessionBrowser"
      @update:open="showSessionBrowser = $event"
      @resume="handleResumeSession"
    />
    <SettingsPanel
      :open="showSettings"
      @update:open="showSettings = $event"
    />
  </div>
</template>
