<script setup lang="ts">
import { ref, computed } from 'vue'
import hljs from 'highlight.js'
import type { ToolUseInfo } from '../types/session'

const props = defineProps<{
  tool: ToolUseInfo
}>()

const outputExpanded = ref(false)
const OUTPUT_LINE_LIMIT = 10

const formattedInput = computed(() => {
  try {
    const parsed = JSON.parse(props.tool.input)
    return JSON.stringify(parsed, null, 2)
  } catch {
    return props.tool.input
  }
})

// Extract file extension from tool input for syntax highlighting
const fileExtension = computed(() => {
  try {
    const parsed = JSON.parse(props.tool.input)
    const filePath = parsed.file_path || parsed.path || parsed.command || ''
    const match = filePath.match(/\.(\w+)$/)
    return match ? match[1] : ''
  } catch {
    return ''
  }
})

const extensionToLang: Record<string, string> = {
  ts: 'typescript', tsx: 'typescript', js: 'javascript', jsx: 'javascript',
  vue: 'xml', html: 'xml', svg: 'xml',
  css: 'css', scss: 'scss', less: 'less',
  go: 'go', rs: 'rust', py: 'python', rb: 'ruby',
  java: 'java', kt: 'kotlin', swift: 'swift',
  json: 'json', yaml: 'yaml', yml: 'yaml', toml: 'toml',
  md: 'markdown', sh: 'bash', bash: 'bash', zsh: 'bash',
  sql: 'sql', graphql: 'graphql',
  dockerfile: 'dockerfile', makefile: 'makefile',
}

const highlightLang = computed(() => extensionToLang[fileExtension.value] || '')

// Strip line number prefixes like "  1→" or "  12→" from Read tool output
function stripLineNumbers(text: string): string {
  const lines = text.split('\n')
  // Check if most lines have the line number prefix pattern
  const prefixPattern = /^\s*\d+[→|]\s?/
  const matchCount = lines.filter(l => prefixPattern.test(l)).length
  if (matchCount > lines.length * 0.5) {
    return lines.map(l => l.replace(prefixPattern, '')).join('\n')
  }
  return text
}

const cleanOutput = computed(() => {
  const raw = props.tool.output || ''
  if (props.tool.name === 'Read' || props.tool.name === 'read') {
    return stripLineNumbers(raw)
  }
  return raw
})

const outputLines = computed(() => cleanOutput.value.split('\n'))
const outputTruncated = computed(() => outputLines.value.length > 20)
const displayedOutput = computed(() => {
  if (!outputTruncated.value || outputExpanded.value) return cleanOutput.value
  return outputLines.value.slice(0, OUTPUT_LINE_LIMIT).join('\n')
})

const highlightedOutput = computed(() => {
  const text = displayedOutput.value || ''
  if (!highlightLang.value || !text) return ''
  try {
    if (hljs.getLanguage(highlightLang.value)) {
      return hljs.highlight(text, { language: highlightLang.value }).value
    }
    return ''
  } catch {
    return ''
  }
})

const isCodeOutput = computed(() =>
  (props.tool.name === 'Read' || props.tool.name === 'read') && highlightedOutput.value.length > 0
)

function copyText(el: Event) {
  const btn = el.target as HTMLButtonElement
  const wrapper = btn.closest('.tool-pre-wrapper')
  const pre = wrapper?.querySelector('pre')
  if (!pre) return
  navigator.clipboard.writeText(pre.textContent || '').then(() => {
    btn.textContent = 'Copied!'
    setTimeout(() => (btn.textContent = 'Copy'), 1500)
  })
}
</script>

<template>
  <div class="collapse collapse-arrow bg-base-200 rounded-lg">
    <input type="checkbox" />
    <div class="collapse-title text-sm font-mono flex items-center gap-2 py-2 min-h-0">
      <span v-if="!tool.output" class="loading loading-spinner loading-xs"></span>
      <span class="badge badge-sm badge-warning">tool</span>
      {{ tool.name }}
      <span v-if="fileExtension" class="text-xs opacity-40">.{{ fileExtension }}</span>
    </div>
    <div class="collapse-content">
      <div v-if="formattedInput" class="tool-pre-wrapper">
        <button class="copy-btn" @click="copyText">Copy</button>
        <pre class="text-xs bg-base-300 rounded p-2 overflow-x-auto whitespace-pre-wrap">{{ formattedInput }}</pre>
      </div>
      <div v-if="tool.output" class="mt-2">
        <div class="text-xs opacity-50 mb-1">Output:</div>
        <div class="tool-pre-wrapper">
          <button class="copy-btn" @click="copyText">Copy</button>
          <pre v-if="isCodeOutput" class="text-xs bg-base-300 rounded p-2 overflow-x-auto"><code class="hljs" v-html="highlightedOutput"></code></pre>
          <pre v-else class="text-xs bg-base-300 rounded p-2 overflow-x-auto whitespace-pre-wrap">{{ displayedOutput }}</pre>
        </div>
        <button
          v-if="outputTruncated && !outputExpanded"
          class="btn btn-ghost btn-xs mt-1"
          @click="outputExpanded = true"
        >
          Show {{ outputLines.length - OUTPUT_LINE_LIMIT }} more lines…
        </button>
        <button
          v-if="outputTruncated && outputExpanded"
          class="btn btn-ghost btn-xs mt-1"
          @click="outputExpanded = false"
        >
          Show less
        </button>
      </div>
    </div>
  </div>
</template>
