<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'

const props = defineProps<{
  open: boolean
  logs: string[]
}>()

defineEmits<{
  'update:open': [value: boolean]
  clear: []
}>()

const logContainer = ref<HTMLElement>()
const autoScroll = ref(true)

watch(() => props.logs.length, async () => {
  if (autoScroll.value) {
    await nextTick()
    if (logContainer.value) {
      logContainer.value.scrollTop = logContainer.value.scrollHeight
    }
  }
})

function formatJson(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

function typeColor(raw: string): string {
  try {
    const obj = JSON.parse(raw)
    switch (obj.type) {
      case 'system': return 'text-info'
      case 'stream_event': return 'text-success'
      case 'assistant': return 'text-primary'
      case 'result': return 'text-warning'
      case 'rate_limit_event': return 'text-base-content/30'
      default: return 'text-base-content/70'
    }
  } catch {
    return 'text-base-content/70'
  }
}

function typeLabel(raw: string): string {
  try {
    const obj = JSON.parse(raw)
    if (obj.type === 'stream_event' && obj.event?.type) {
      return `stream_event.${obj.event.type}`
    }
    return obj.type || '???'
  } catch {
    return '???'
  }
}
</script>

<template>
  <div class="w-[500px] shrink-0 bg-base-200 border-l border-base-300 flex flex-col">
      <!-- Header -->
      <div class="flex items-center justify-between px-3 py-2 bg-base-300 border-b border-base-300">
        <span class="font-bold text-sm">Debug Log</span>
        <div class="flex items-center gap-2">
          <label class="label cursor-pointer gap-1">
            <span class="label-text text-xs">Auto-scroll</span>
            <input v-model="autoScroll" type="checkbox" class="checkbox checkbox-xs" />
          </label>
          <button class="btn btn-ghost btn-xs" @click="$emit('clear')">Clear</button>
          <button class="btn btn-ghost btn-xs" @click="$emit('update:open', false)">Close</button>
        </div>
      </div>

      <!-- Log entries -->
      <div ref="logContainer" class="flex-1 overflow-y-auto p-2 space-y-1 font-mono text-xs">
        <div v-if="logs.length === 0" class="text-center text-base-content/30 py-8">
          No events yet. Send a message to see Claude's raw JSON output.
        </div>
        <details v-for="(log, i) in logs" :key="i" class="bg-base-300 rounded px-2 py-1">
          <summary class="cursor-pointer select-none" :class="typeColor(log)">
            <span class="opacity-50 mr-1">{{ i }}</span>
            {{ typeLabel(log) }}
          </summary>
          <pre class="mt-1 whitespace-pre-wrap break-all text-base-content/80 max-h-64 overflow-y-auto">{{ formatJson(log) }}</pre>
        </details>
      </div>

      <!-- Footer -->
      <div class="px-3 py-1 text-xs text-base-content/40 bg-base-300 border-t border-base-300">
        {{ logs.length }} events
      </div>
  </div>
</template>
