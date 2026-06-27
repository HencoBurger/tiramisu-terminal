<script setup lang="ts">
import { computed } from 'vue'
import type { ChatMessage } from '../types/session'
import { renderMarkdown } from '../utils/markdown'
import ToolUseBlock from './ToolUseBlock.vue'

const props = withDefaults(defineProps<{
  message: ChatMessage
  assistantLabel?: string
}>(), { assistantLabel: 'Claude' })

const renderedContent = computed(() => renderMarkdown(props.message.content))
const hasTextContent = computed(() => props.message.content.trim().length > 0)
const hasReasoning = computed(() => (props.message.reasoning || '').trim().length > 0)
const renderedReasoning = computed(() => renderMarkdown(props.message.reasoning || ''))
const showNoResponse = computed(() =>
  !hasTextContent.value && !hasReasoning.value && props.message.toolUse.length === 0 && !props.message.isStreaming,
)
const isUser = computed(() => props.message.role === 'user')
const isSystem = computed(() => props.message.role === 'system')
const timeStr = computed(() => {
  const d = new Date(props.message.timestamp)
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
})
</script>

<template>
  <!-- System message: centered info alert -->
  <div v-if="isSystem" class="flex justify-center my-2">
    <div class="alert alert-info max-w-2xl shadow-sm">
      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-current shrink-0 w-5 h-5"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
      <div class="prose prose-sm prose-invert break-words" v-html="renderedContent" />
    </div>
  </div>

  <!-- User message: compact bubble, right-aligned -->
  <div v-else-if="isUser" class="chat chat-end">
    <div class="chat-header text-xs opacity-50 mb-1">
      You
      <time class="ml-1">{{ timeStr }}</time>
    </div>
    <div
      v-if="hasTextContent"
      class="chat-bubble chat-bubble-primary prose prose-sm prose-invert break-words"
      v-html="renderedContent"
    />
  </div>

  <!-- Assistant message: full-width card style -->
  <div v-else class="assistant-message">
    <div class="text-xs opacity-40 mb-1.5 flex items-center gap-1.5">
      <span class="inline-block w-4 h-4 rounded-full bg-gradient-to-br from-orange-400 to-amber-600 shrink-0"></span>
      {{ assistantLabel }}
      <time class="ml-1">{{ timeStr }}</time>
    </div>
    <!-- Reasoning is collapsed to a small "Thinking" disclosure, never dumped inline. -->
    <details v-if="hasReasoning" class="text-xs opacity-60 mb-1.5">
      <summary class="cursor-pointer select-none">💭 Thinking</summary>
      <div class="assistant-content prose prose-sm prose-invert break-words mt-1 pl-2 border-l border-base-content/20" v-html="renderedReasoning" />
    </details>

    <div
      v-if="hasTextContent"
      class="assistant-content prose prose-sm prose-invert break-words"
      v-html="renderedContent"
    />
    <div v-else-if="showNoResponse" class="text-xs opacity-40 italic">(no response)</div>

    <div v-if="message.toolUse.length > 0" class="mt-2 space-y-1.5">
      <ToolUseBlock v-for="tool in message.toolUse" :key="tool.id" :tool="tool" />
    </div>
    <div v-if="message.isStreaming" class="mt-2">
      <span class="loading loading-dots loading-sm text-base-content/50"></span>
    </div>
  </div>
</template>
