<script setup lang="ts">
import { ref, computed, nextTick } from 'vue'
import { slashCommands, type SlashCommand } from '../types/slashCommands'

const props = defineProps<{
  disabled?: boolean
  placeholder?: string
}>()

const emit = defineEmits<{
  send: [message: string, images: string[]]
  command: [action: string]
}>()

const message = ref('')
const textareaEl = ref<HTMLTextAreaElement>()
const menuEl = ref<HTMLUListElement>()
const fileInputEl = ref<HTMLInputElement>()
const attachedImages = ref<string[]>([])

function fileToDataURL(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string)
    reader.onerror = reject
    reader.readAsDataURL(file)
  })
}

async function addFiles(files: File[]) {
  for (const f of files) {
    if (f.type.startsWith('image/')) {
      try {
        attachedImages.value.push(await fileToDataURL(f))
      } catch {
        // ignore unreadable file
      }
    }
  }
}

function handlePaste(e: ClipboardEvent) {
  const items = e.clipboardData?.items
  if (!items) return
  const imgs: File[] = []
  for (const it of Array.from(items)) {
    if (it.kind === 'file' && it.type.startsWith('image/')) {
      const f = it.getAsFile()
      if (f) imgs.push(f)
    }
  }
  if (imgs.length) {
    e.preventDefault()
    addFiles(imgs)
  }
}

function onFilePick(e: Event) {
  const input = e.target as HTMLInputElement
  if (input.files) addFiles(Array.from(input.files))
  input.value = ''
}

function removeImage(i: number) {
  attachedImages.value.splice(i, 1)
}

const canSend = computed(
  () => !props.disabled && (message.value.trim().length > 0 || attachedImages.value.length > 0),
)

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
  if ((!text && attachedImages.value.length === 0) || props.disabled) return
  emit('send', text, attachedImages.value.slice())
  message.value = ''
  attachedImages.value = []
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

    <div class="p-3 bg-base-200 border-t border-base-300">
      <!-- Attached image thumbnails -->
      <div v-if="attachedImages.length" class="flex flex-wrap gap-2 mb-2">
        <div v-for="(img, i) in attachedImages" :key="i" class="relative">
          <img :src="img" class="w-16 h-16 object-cover rounded border border-base-300" alt="attachment" />
          <button
            class="absolute -top-1.5 -right-1.5 btn btn-xs btn-circle btn-error"
            title="Remove"
            @click="removeImage(i)"
          >×</button>
        </div>
      </div>

      <div class="flex gap-2 items-end">
        <input ref="fileInputEl" type="file" accept="image/*" multiple class="hidden" @change="onFilePick" />
        <button
          class="btn btn-ghost btn-sm btn-square"
          :disabled="disabled"
          title="Attach image"
          @click="fileInputEl?.click()"
        >
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="w-5 h-5">
            <rect x="3" y="3" width="18" height="18" rx="2" />
            <circle cx="8.5" cy="8.5" r="1.5" />
            <path d="M21 15l-5-5L5 21" />
          </svg>
        </button>
        <textarea
          ref="textareaEl"
          v-model="message"
          :placeholder="placeholder || 'Send a message... (type / for commands, paste an image)'"
          :disabled="disabled"
          class="textarea textarea-bordered flex-1 min-h-10 max-h-48 resize-none leading-normal"
          rows="1"
          @keydown="handleKeydown"
          @input="handleInput"
          @paste="handlePaste"
        />
        <button
          class="btn btn-primary btn-sm"
          :disabled="!canSend"
          @click="send"
        >
          Send
        </button>
      </div>
    </div>
  </div>
</template>
