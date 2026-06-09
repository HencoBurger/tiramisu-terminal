<script setup lang="ts">
import { ref, computed } from 'vue'
import type { TabState } from '../types/session'

const props = defineProps<{
  tab: TabState
  isActive: boolean
}>()

const emit = defineEmits<{
  select: []
  close: []
  rename: [name: string]
  contextmenu: [event: MouseEvent]
}>()

const isRenaming = ref(false)
const renameInput = ref('')
const inputEl = ref<HTMLInputElement>()

const badgeClass = computed(() => {
  switch (props.tab.status) {
    case 'thinking': return 'badge-success animate-pulse'
    case 'tool_use': return 'badge-success animate-pulse'
    case 'done': return 'badge-success'
    case 'error': return 'badge-error'
    default: return ''
  }
})

const badgeText = computed(() => {
  switch (props.tab.status) {
    case 'thinking': return '●'
    case 'tool_use': return '●'
    case 'done': return '✓'
    case 'error': return '!'
    default: return ''
  }
})

const showBadge = computed(() => props.tab.status !== 'idle')

function startRename() {
  renameInput.value = props.tab.name
  isRenaming.value = true
  setTimeout(() => inputEl.value?.select(), 0)
}

function finishRename() {
  if (renameInput.value.trim()) {
    emit('rename', renameInput.value.trim())
  }
  isRenaming.value = false
}
</script>

<template>
  <div
    class="tab-item flex items-center gap-1 px-3 py-1.5 cursor-pointer select-none rounded-t-lg text-sm border-b-2 min-w-0 max-w-48"
    :class="isActive ? 'bg-base-100 border-primary text-base-content' : 'bg-base-300 border-transparent text-base-content/60 hover:text-base-content/80'"
    @click="emit('select')"
    @dblclick="startRename"
    @contextmenu.prevent="emit('contextmenu', $event)"
  >
    <span v-if="tab.type === 'terminal'" class="text-xs opacity-60 font-mono">&gt;_</span>
    <span v-if="tab.activity" class="inline-block w-2 h-2 rounded-full bg-success animate-pulse shrink-0" />
    <span v-else-if="tab.status === 'thinking' || tab.status === 'tool_use'" class="inline-block w-2 h-2 rounded-full bg-success animate-pulse shrink-0" />
    <span v-else-if="tab.status === 'error'" class="inline-block w-2 h-2 rounded-full bg-error shrink-0" />

    <input
      v-if="isRenaming"
      ref="inputEl"
      v-model="renameInput"
      class="input input-xs input-ghost w-24 p-0 min-h-0 h-5"
      @blur="finishRename"
      @keydown.enter="finishRename"
      @keydown.escape="isRenaming = false"
      @click.stop
    />
    <span v-else class="truncate">{{ tab.name }}</span>

    <button
      class="btn btn-ghost btn-xs px-1 min-h-0 h-5 opacity-0 group-hover:opacity-100 hover:opacity-100 ml-auto"
      :class="{ 'opacity-100': isActive }"
      @click.stop="emit('close')"
    >
      ✕
    </button>
  </div>
</template>
