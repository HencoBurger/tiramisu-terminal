<script setup lang="ts">
import { ref, computed, nextTick } from 'vue'
import { slashCommands, type SlashCommand } from '../types/slashCommands'

const props = defineProps<{
  disabled?: boolean
  placeholder?: string
}>()

const emit = defineEmits<{
  send: [message: string]
  command: [action: string]
}>()

const message = ref('')
const textareaEl = ref<HTMLTextAreaElement>()
const menuEl = ref<HTMLUListElement>()

// Explicit menu state — not derived from message to avoid reactivity flicker
const menuOpen = ref(false)
const menuItems = ref<SlashCommand[]>([])
const menuIndex = ref(0)

function updateMenu() {
  const val = message.value
  if (val.startsWith('/') && !val.includes(' ')) {
    const q = val.toLowerCase()
    const matches = slashCommands.filter(c => c.name.startsWith(q))
    if (matches.length > 0) {
      menuItems.value = matches
      menuIndex.value = 0
      menuOpen.value = true
      return
    }
  }
  menuOpen.value = false
}

function handleKeydown(e: KeyboardEvent) {
  if (menuOpen.value && menuItems.value.length > 0) {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      e.stopImmediatePropagation()
      menuIndex.value = (menuIndex.value + 1) % menuItems.value.length
      scrollToItem()
      return
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault()
      e.stopImmediatePropagation()
      menuIndex.value = (menuIndex.value - 1 + menuItems.value.length) % menuItems.value.length
      scrollToItem()
      return
    }
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      e.stopImmediatePropagation()
      pickItem(menuItems.value[menuIndex.value])
      return
    }
    if (e.key === 'Tab') {
      e.preventDefault()
      e.stopImmediatePropagation()
      pickItem(menuItems.value[menuIndex.value])
      return
    }
    if (e.key === 'Escape') {
      e.preventDefault()
      menuOpen.value = false
      message.value = ''
      return
    }
  }

  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    send()
  }
}

function scrollToItem() {
  nextTick(() => {
    const items = menuEl.value?.querySelectorAll('li')
    items?.[menuIndex.value]?.scrollIntoView({ block: 'nearest' })
  })
}

function pickItem(cmd: SlashCommand) {
  menuOpen.value = false
  message.value = ''
  if (textareaEl.value) {
    textareaEl.value.style.height = 'auto'
  }
  emit('command', cmd.action)
}

function send() {
  const text = message.value.trim()
  if (!text || props.disabled) return
  emit('send', text)
  message.value = ''
  if (textareaEl.value) {
    textareaEl.value.style.height = 'auto'
  }
}

function handleInput() {
  // Auto resize
  const el = textareaEl.value
  if (el) {
    el.style.height = 'auto'
    el.style.height = Math.min(el.scrollHeight, 200) + 'px'
  }
  // Update menu based on current input
  updateMenu()
}
</script>

<template>
  <div class="relative">
    <!-- Slash command menu -->
    <div
      v-if="menuOpen"
      class="absolute bottom-full left-0 right-0 mx-3 mb-1 z-50"
    >
      <ul ref="menuEl" class="menu bg-base-200 rounded-box shadow-lg border border-base-300 p-1 max-h-64 overflow-y-auto">
        <li
          v-for="(cmd, i) in menuItems"
          :key="cmd.action"
        >
          <a
            class="flex justify-between gap-4 py-1.5 px-3"
            :class="{ 'active': i === menuIndex }"
            @click="pickItem(cmd)"
            @mouseenter="menuIndex = i"
          >
            <span class="font-mono text-sm">{{ cmd.name }}</span>
            <span class="text-xs opacity-60">{{ cmd.description }}</span>
          </a>
        </li>
      </ul>
    </div>

    <div class="flex gap-2 items-end p-3 bg-base-200 border-t border-base-300">
      <textarea
        ref="textareaEl"
        v-model="message"
        :placeholder="placeholder || 'Send a message... (type / for commands)'"
        :disabled="disabled"
        class="textarea textarea-bordered flex-1 min-h-10 max-h-48 resize-none leading-normal"
        rows="1"
        @keydown="handleKeydown"
        @input="handleInput"
      />
      <button
        class="btn btn-primary btn-sm"
        :disabled="disabled || !message.trim()"
        @click="send"
      >
        Send
      </button>
    </div>
  </div>
</template>
