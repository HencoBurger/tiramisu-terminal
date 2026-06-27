<script lang="ts" setup>
import { ref, watch, onMounted, onUnmounted } from 'vue'
import type * as Monaco from 'monaco-editor'
import monaco from './monacoSetup'
import { guessLanguage } from './language'

const props = defineProps<{
  activePath: string
  // Initial on-open content for activePath; only read when a model is first created.
  content: string
}>()

const emit = defineEmits<{
  'update:content': [value: string]
  save: []
}>()

const containerRef = ref<HTMLElement>()
let editor: Monaco.editor.IStandaloneCodeEditor | null = null
const models = new Map<string, Monaco.editor.ITextModel>()
let resizeObserver: ResizeObserver | null = null

function modelFor(path: string, content: string): Monaco.editor.ITextModel {
  let model = models.get(path)
  if (!model) {
    model = monaco.editor.createModel(content, guessLanguage(path))
    model.onDidChangeContent(() => emit('update:content', model!.getValue()))
    models.set(path, model)
  }
  return model
}

onMounted(() => {
  if (!containerRef.value) return
  editor = monaco.editor.create(containerRef.value, {
    theme: 'vs-dark',
    automaticLayout: true,
    minimap: { enabled: true },
    fontSize: 14,
    lineNumbers: 'on',
    roundedSelection: false,
    scrollBeyondLastLine: false,
    wordWrap: 'off',
    tabSize: 2,
  })
  editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => emit('save'))

  if (props.activePath) {
    editor.setModel(modelFor(props.activePath, props.content))
  }

  resizeObserver = new ResizeObserver(() => editor?.layout())
  resizeObserver.observe(containerRef.value)
})

watch(() => props.activePath, (path) => {
  if (!editor) return
  editor.setModel(path ? modelFor(path, props.content) : null)
})

// Drop a model when its file tab is closed upstream.
function disposeModel(path: string) {
  const model = models.get(path)
  if (model) {
    model.dispose()
    models.delete(path)
  }
}

// Re-measure after becoming visible (Monaco mis-sizes if created/shown while hidden).
function layout() {
  editor?.layout()
}

defineExpose({ disposeModel, layout })

onUnmounted(() => {
  if (resizeObserver) resizeObserver.disconnect()
  models.forEach((m) => m.dispose())
  models.clear()
  if (editor) {
    editor.dispose()
    editor = null
  }
})
</script>

<template>
  <div ref="containerRef" class="w-full h-full" />
</template>
