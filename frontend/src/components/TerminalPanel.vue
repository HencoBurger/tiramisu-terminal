<script lang="ts">
// Module-level: single global listener, dispatches to per-tab handlers
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'

const outputHandlers = new Map<string, (sessionId: string, data: string) => void>()
let listenerRegistered = false

function ensureGlobalListener() {
  if (listenerRegistered) return
  listenerRegistered = true
  EventsOn('terminal:output', (sessionId: string, data: string) => {
    const handler = outputHandlers.get(sessionId)
    if (handler) handler(sessionId, data)
  })
}
</script>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { TerminalStart, TerminalInput, TerminalResize, TerminalStop } from '../../wailsjs/go/main/App'
import { useTabs } from '../composables/useTabs'
import { useConfig } from '../composables/useConfig'
import { useSound } from '../composables/useSound'
import type { TabState } from '../types/session'
import WorkDirPicker from './WorkDirPicker.vue'

const props = defineProps<{
  tab: TabState
}>()

const { activeTabId, setTabWorkDir, setTabActivity } = useTabs()
const { config } = useConfig()
const { play } = useSound()

const containerRef = ref<HTMLElement>()
const showWorkDirPicker = ref(false)
let terminal: Terminal | null = null
let fitAddon: FitAddon | null = null
let resizeObserver: ResizeObserver | null = null
let started = false
let idleTimer: ReturnType<typeof setTimeout> | null = null
let suppressActivity = false

function startTerminal() {
  if (started || !containerRef.value || !props.tab.workDir) return
  started = true

  terminal = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    fontFamily: 'monospace',
    theme: {
      background: '#1d232a',
      foreground: '#a6adbb',
    },
  })
  fitAddon = new FitAddon()
  terminal.loadAddon(fitAddon)
  terminal.open(containerRef.value)
  fitAddon.fit()

  const cols = terminal.cols
  const rows = terminal.rows

  TerminalStart(props.tab.id, cols, rows, props.tab.workDir).catch((err: any) => {
    console.error('Failed to start terminal:', err)
  })

  terminal.onData((data: string) => {
    const encoded = btoa(data)
    TerminalInput(props.tab.id, encoded).catch(() => {})
  })

  resizeObserver = new ResizeObserver(() => {
    if (!fitAddon || !terminal) return
    fitAddon.fit()
    TerminalResize(props.tab.id, terminal.cols, terminal.rows).catch(() => {})
  })
  resizeObserver.observe(containerRef.value)
}

function handleOutput(sessionId: string, base64Data: string) {
  if (!terminal) return

  const bytes = Uint8Array.from(atob(base64Data), c => c.charCodeAt(0))
  terminal.write(bytes)

  if (!suppressActivity) {
    setTabActivity(props.tab.id, true)
    resetIdleTimer()
  }
}

function resetIdleTimer() {
  if (idleTimer) clearTimeout(idleTimer)
  idleTimer = setTimeout(() => {
    setTabActivity(props.tab.id, false)

    if (props.tab.id !== activeTabId.value) {
      const soundName = props.tab.soundOverride || config.value.defaultSound
      if (soundName) play(soundName)
    }
  }, 3000)
}

function handleWorkDirSelected(dir: string) {
  setTabWorkDir(props.tab.id, dir)
}

watch(() => props.tab.workDir, (newDir) => {
  if (newDir && !started) {
    nextTick(() => startTerminal())
  }
})

watch(() => activeTabId.value, (newId) => {
  if (newId === props.tab.id) {
    setTabActivity(props.tab.id, false)
    // Suppress activity briefly so the resize/redraw from fit() doesn't trigger the dot
    suppressActivity = true
    setTimeout(() => { suppressActivity = false }, 500)
    if (terminal && fitAddon) {
      nextTick(() => {
        terminal!.focus()
        fitAddon!.fit()
      })
    }
  }
})

onMounted(() => {
  ensureGlobalListener()
  outputHandlers.set(props.tab.id, handleOutput)
  if (props.tab.workDir) {
    nextTick(() => startTerminal())
  } else {
    showWorkDirPicker.value = true
  }
})

onUnmounted(() => {
  outputHandlers.delete(props.tab.id)
  if (idleTimer) clearTimeout(idleTimer)
  if (resizeObserver) resizeObserver.disconnect()
  if (started) {
    TerminalStop(props.tab.id).catch(() => {})
  }
  if (terminal) {
    terminal.dispose()
    terminal = null
  }
})
</script>

<template>
  <div class="h-full flex flex-col">
    <div v-if="!tab.workDir" class="flex items-center justify-center h-full text-base-content/30">
      <div class="text-center">
        <p class="text-lg mb-2">Terminal</p>
        <p class="text-sm">
          Select a working directory to get started
          <button class="btn btn-sm btn-outline ml-2" @click="showWorkDirPicker = true">
            Choose Directory
          </button>
        </p>
      </div>
    </div>
    <div v-else ref="containerRef" class="flex-1 overflow-hidden p-1" />
    <WorkDirPicker
      :open="showWorkDirPicker"
      @update:open="showWorkDirPicker = $event"
      @select="handleWorkDirSelected"
    />
  </div>
</template>

<style>
@import '@xterm/xterm/css/xterm.css';
</style>
