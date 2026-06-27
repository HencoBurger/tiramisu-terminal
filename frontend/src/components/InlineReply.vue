<script setup lang="ts">
import { ref, computed, nextTick, onMounted, watch } from 'vue'

const props = defineProps<{
  message: string
}>()

const emit = defineEmits<{
  send: [message: string]
}>()

const answers = ref<string[]>([])
const selectedOptions = ref<Set<number>>(new Set())
const selectedQuickActions = ref<Set<number>>(new Set())
const firstInput = ref<HTMLInputElement>()

// Remove fenced code blocks and inline code so code (e.g. a `cond ? a : b` ternary)
// is never mistaken for a question/option.
function stripCode(text: string): string {
  return text
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/`[^`]*`/g, ' ')
}

function stripMarkdown(text: string): string {
  return text
    .replace(/\*\*(.+?)\*\*/g, '$1')
    .replace(/\*(.+?)\*/g, '$1')
    .replace(/__(.+?)__/g, '$1')
    .replace(/_(.+?)_/g, '$1')
    .replace(/`(.+?)`/g, '$1')
    .replace(/\[(.+?)\]\(.+?\)/g, '$1')
    .trim()
}

function extractQuestions(text: string): string[] {
  const found: string[] = []
  const matches = text.match(/[^.!?\n]*\?/g)
  if (matches) {
    for (const m of matches) {
      const clean = stripMarkdown(m.trim())
      if (clean.length > 5) {
        found.push(clean)
      }
    }
  }
  return found
}

function extractOptions(text: string): string[] {
  const lines = text.split('\n')
  const found: string[] = []

  for (const line of lines) {
    const trimmed = line.trim()
    if (!trimmed) continue
    const match = trimmed.match(/^(?:\d+[.)]\s*|[-•]\s+|\*\s+)(.+)/)
    if (match) {
      const cleaned = stripMarkdown(match[1].trim())
      if (cleaned) {
        found.push(cleaned)
      }
    }
  }

  return found
}

function isPermissionRequest(text: string): boolean {
  const lower = stripMarkdown(text).toLowerCase()
  if (/\b(approve|permission|allow|authorize)\b/.test(lower) &&
      /\b(could you|can you|would you|please|do you)\b/.test(lower)) {
    return true
  }
  if (/\b(shall i|should i|do you want me to|would you like me to|can i|may i)\b/.test(lower) &&
      /\b(proceed|continue|go ahead|create|write|delete|run|execute|install|update|modify)\b/.test(lower)) {
    return true
  }
  if (/\b(is that ok|sound good|look right|that work|go ahead)\b/.test(lower) &&
      /\?/.test(lower)) {
    return true
  }
  return false
}

const cleanMessage = computed(() => stripCode(props.message))
const questions = computed(() => extractQuestions(cleanMessage.value))
// Only show selectable options if the message actually asks a question
const options = computed(() => questions.value.length > 0 ? extractOptions(cleanMessage.value) : [])
const isPermission = computed(() => isPermissionRequest(cleanMessage.value))

const quickActions = computed(() => {
  if (options.value.length > 0) return []
  if (isPermission.value) {
    return ['Yes', 'No', 'Yes, and allow for this session']
  }
  return []
})

const hasQuestions = computed(() =>
  questions.value.length > 0 || options.value.length > 0 || quickActions.value.length > 0
)

const hasAnySelection = computed(() => {
  if (selectedOptions.value.size > 0) return true
  if (selectedQuickActions.value.size > 0) return true
  if (answers.value.some(a => a.trim())) return true
  return false
})

onMounted(() => {
  answers.value = questions.value.map(() => '')
  nextTick(() => firstInput.value?.focus())
})

watch(() => props.message, () => {
  answers.value = questions.value.map(() => '')
  selectedOptions.value = new Set()
  selectedQuickActions.value = new Set()
})

function toggleOption(index: number) {
  const s = new Set(selectedOptions.value)
  if (s.has(index)) {
    s.delete(index)
  } else {
    s.add(index)
  }
  selectedOptions.value = s
}

function toggleQuickAction(index: number) {
  const s = new Set(selectedQuickActions.value)
  if (s.has(index)) {
    s.delete(index)
  } else {
    s.add(index)
  }
  selectedQuickActions.value = s
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    send()
  }
}

function send() {
  const parts: string[] = []

  // Collect selected quick actions
  for (const i of selectedQuickActions.value) {
    parts.push(quickActions.value[i])
  }

  // Collect selected options
  for (const i of selectedOptions.value) {
    parts.push(options.value[i])
  }

  // Collect answered questions
  for (let i = 0; i < answers.value.length; i++) {
    if (answers.value[i].trim()) {
      parts.push(answers.value[i].trim())
    }
  }

  if (parts.length === 0) return

  emit('send', parts.join(', '))
  answers.value = questions.value.map(() => '')
  selectedOptions.value = new Set()
  selectedQuickActions.value = new Set()
}
</script>

<template>
  <div v-if="hasQuestions" class="mt-3 max-w-xl space-y-3">
    <!-- Quick action buttons (permissions, yes/no) — toggle select -->
    <div v-if="quickActions.length > 0" class="flex flex-wrap gap-2">
      <button
        v-for="(action, i) in quickActions"
        :key="'qa-' + i"
        class="btn btn-sm"
        :class="selectedQuickActions.has(i)
          ? (action === 'Yes' ? 'btn-success' : action === 'No' ? 'btn-error' : 'btn-primary')
          : 'btn-outline'"
        @click="toggleQuickAction(i)"
      >
        {{ action }}
      </button>
    </div>

    <!-- Selectable options from lists — toggle select -->
    <div v-if="options.length > 0" class="flex flex-wrap gap-2">
      <button
        v-for="(opt, i) in options"
        :key="'opt-' + i"
        class="btn btn-sm"
        :class="selectedOptions.has(i) ? 'btn-primary' : 'btn-outline'"
        @click="toggleOption(i)"
      >
        {{ opt }}
      </button>
    </div>

    <!-- Question input fields -->
    <div v-if="questions.length > 0" class="space-y-2">
      <div v-for="(q, i) in questions" :key="'q-' + i" class="flex flex-col gap-1">
        <label class="text-sm text-base-content/70 font-medium">{{ q }}</label>
        <input
          :ref="(el) => { if (i === 0) firstInput = el as HTMLInputElement }"
          v-model="answers[i]"
          type="text"
          placeholder="Answer..."
          class="input input-bordered input-sm w-full"
          @keydown="handleKeydown"
        />
      </div>
    </div>

    <!-- Single send button for everything -->
    <button
      class="btn btn-primary btn-sm"
      :disabled="!hasAnySelection"
      @click="send"
    >
      Send
    </button>
  </div>
</template>
