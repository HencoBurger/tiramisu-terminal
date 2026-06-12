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
import { SerializeAddon } from '@xterm/addon-serialize'
import { TerminalStart, TerminalInput, TerminalResize, TerminalStop, TerminalSaveScrollback, TerminalLoadScrollback } from '../../wailsjs/go/main/App'
import { ClipboardGetText, ClipboardSetText } from '../../wailsjs/runtime/runtime'
import { useTabs } from '../composables/useTabs'
import { useConfig } from '../composables/useConfig'
import { useSound } from '../composables/useSound'
import type { TabState } from '../types/session'
import WorkDirPicker from './WorkDirPicker.vue'

const props = defineProps<{
  tab: TabState
}>()

const { activeTabId, setTabWorkDir, setTabActivity } = useTabs()
const { effectiveConfig, maybeSetDefaultWorkDir } = useConfig()
const { play } = useSound()

const containerRef = ref<HTMLElement>()
const showWorkDirPicker = ref(false)
const contextMenu = ref<{ x: number; y: number; hasCopy: boolean; hasPaste: boolean } | null>(null)
let terminal: Terminal | null = null
let fitAddon: FitAddon | null = null
let serializeAddon: SerializeAddon | null = null
let resizeObserver: ResizeObserver | null = null
let started = false
let idleTimer: ReturnType<typeof setTimeout> | null = null
let suppressActivity = false

async function startTerminal() {
  if (started || !containerRef.value || !props.tab.workDir) return
  started = true

  terminal = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    fontFamily: 'monospace',
    scrollback: 5000,
    theme: {
      background: '#1d232a',
      foreground: '#a6adbb',
    },
  })
  fitAddon = new FitAddon()
  serializeAddon = new SerializeAddon()
  terminal.loadAddon(fitAddon)
  terminal.loadAddon(serializeAddon)
  terminal.open(containerRef.value)
  fitAddon.fit()

  const cols = terminal.cols
  const rows = terminal.rows

  // Restore the previous session's visual scrollback (if any) before the fresh
  // shell starts printing, so the old content sits above the new live prompt.
  try {
    const saved = await TerminalLoadScrollback(props.tab.id)
    if (saved && terminal) {
      const bytes = Uint8Array.from(atob(saved), c => c.charCodeAt(0))
      terminal.write(bytes)
      terminal.write('\r\n\x1b[2m──── session restored ────\x1b[0m\r\n')
    }
  } catch {}

  TerminalStart(props.tab.id, cols, rows, props.tab.workDir).catch((err: any) => {
    console.error('Failed to start terminal:', err)
  })

  terminal.onData((data: string) => {
    const encoded = btoa(data)
    TerminalInput(props.tab.id, encoded).catch(() => {})
  })

  // Right-click: show context menu
  const xtermEl = terminal.element
  if (xtermEl) {
    xtermEl.addEventListener('contextmenu', async (e: MouseEvent) => {
      e.preventDefault()
      e.stopPropagation()
      if (!terminal) return

      const hasCopy = !!terminal.getSelection()
      let hasPaste = false
      try {
        const text = await ClipboardGetText()
        hasPaste = !!text
      } catch {}

      if (!hasCopy && !hasPaste) return

      contextMenu.value = { x: e.clientX, y: e.clientY, hasCopy, hasPaste }
    }, true)
  }

  // Keyboard shortcuts
  terminal.attachCustomKeyEventHandler((e: KeyboardEvent) => {
    if (e.type !== 'keydown') return true

    // Ctrl+Shift+C → copy
    if (e.ctrlKey && e.shiftKey && e.key === 'C') {
      e.preventDefault()
      copySelection()
      return false
    }
    // Ctrl+Shift+V → paste
    if (e.ctrlKey && e.shiftKey && e.key === 'V') {
      e.preventDefault()
      pasteClipboard()
      return false
    }
    // Ctrl+C with selection → copy instead of SIGINT
    if (e.ctrlKey && !e.shiftKey && e.key === 'c' && terminal?.hasSelection()) {
      e.preventDefault()
      copySelection()
      return false
    }
    // Ctrl+V → paste
    if (e.ctrlKey && !e.shiftKey && e.key === 'v') {
      e.preventDefault()
      pasteClipboard()
      return false
    }

    return true
  })

  resizeObserver = new ResizeObserver(() => {
    if (!fitAddon || !terminal) return
    fitAddon.fit()
    TerminalResize(props.tab.id, terminal.cols, terminal.rows).catch(() => {})
  })
  resizeObserver.observe(containerRef.value)
}

function copySelection() {
  if (!terminal) return
  const selection = terminal.getSelection()
  if (selection) {
    ClipboardSetText(selection)
    terminal.clearSelection()
  }
}

function pasteClipboard() {
  ClipboardGetText().then((text) => {
    if (text) {
      const encoded = btoa(text)
      TerminalInput(props.tab.id, encoded).catch(() => {})
    } else {
      // No text in clipboard (e.g. an image) — forward a literal Ctrl+V so the
      // running program can read the clipboard itself (Claude Code image paste)
      TerminalInput(props.tab.id, btoa('\x16')).catch(() => {})
    }
  }).catch(() => {})
}

function closeContextMenu() {
  contextMenu.value = null
}

function handleContextCopy() {
  copySelection()
  closeContextMenu()
}

function handleContextPaste() {
  pasteClipboard()
  closeContextMenu()
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

function bytesToBase64(bytes: Uint8Array): string {
  // Build the binary string in chunks — spreading a large array into
  // String.fromCharCode overflows the call stack.
  let binary = ''
  const chunk = 0x8000
  for (let i = 0; i < bytes.length; i += chunk) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunk))
  }
  return btoa(binary)
}

function saveScrollback() {
  if (!serializeAddon) return
  try {
    const snapshot = serializeAddon.serialize()
    if (!snapshot) return
    // Encode the UTF-8 bytes so non-ASCII characters (box-drawing, emoji, etc.)
    // survive the round-trip; xterm.write() decodes the bytes back as UTF-8.
    const encoded = bytesToBase64(new TextEncoder().encode(snapshot))
    TerminalSaveScrollback(props.tab.id, encoded).catch(() => {})
  } catch {}
}

function resetIdleTimer() {
  if (idleTimer) clearTimeout(idleTimer)
  idleTimer = setTimeout(() => {
    setTabActivity(props.tab.id, false)
    // Output has settled — snapshot the buffer for restore on next launch.
    saveScrollback()

    if (props.tab.id !== activeTabId.value) {
      const soundName = props.tab.soundOverride || effectiveConfig.value.defaultSound
      if (soundName) play(soundName)
    }
  }, 3000)
}

function handleWorkDirSelected(dir: string) {
  setTabWorkDir(props.tab.id, dir)
  maybeSetDefaultWorkDir(dir)
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

function onDocumentClick() {
  if (contextMenu.value) contextMenu.value = null
}

onMounted(() => {
  document.addEventListener('click', onDocumentClick)
  ensureGlobalListener()
  outputHandlers.set(props.tab.id, handleOutput)
  if (props.tab.workDir) {
    nextTick(() => startTerminal())
  } else {
    showWorkDirPicker.value = true
  }
})

onUnmounted(() => {
  document.removeEventListener('click', onDocumentClick)
  outputHandlers.delete(props.tab.id)
  if (idleTimer) clearTimeout(idleTimer)
  if (resizeObserver) resizeObserver.disconnect()
  if (started) {
    // Best-effort final snapshot before the panel goes away.
    saveScrollback()
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
    <!-- Terminal context menu -->
    <div
      v-if="contextMenu"
      class="fixed z-50"
      :style="{ left: contextMenu.x + 'px', top: contextMenu.y + 'px' }"
    >
      <ul class="menu bg-base-200 rounded-box shadow-lg w-48 p-1">
        <li v-if="contextMenu.hasCopy"><a @click="handleContextCopy">Copy</a></li>
        <li v-if="contextMenu.hasPaste"><a @click="handleContextPaste">Paste</a></li>
      </ul>
    </div>

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
