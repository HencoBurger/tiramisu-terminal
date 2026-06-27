<script lang="ts" setup>
import { ref, watch, onMounted, onUnmounted } from 'vue'
import type * as Monaco from 'monaco-editor'
import monaco from './monacoSetup'
import { guessLanguage } from './language'
import type { RunAction } from './run/types'

const props = defineProps<{
  activePath: string
  // Initial on-open content for activePath; only read when a model is first created.
  content: string
  runActions?: RunAction[]
}>()

const emit = defineEmits<{
  'update:content': [value: string]
  save: []
  'run-action': [action: RunAction]
}>()

const containerRef = ref<HTMLElement>()
let editor: Monaco.editor.IStandaloneCodeEditor | null = null
const models = new Map<string, Monaco.editor.ITextModel>()
let resizeObserver: ResizeObserver | null = null

// Gutter "run" glyphs.
let runDecorations: Monaco.editor.IEditorDecorationsCollection | null = null
let lineToAction = new Map<number, RunAction>()

function modelFor(path: string, content: string): Monaco.editor.ITextModel {
  let model = models.get(path)
  if (!model) {
    model = monaco.editor.createModel(content, guessLanguage(path))
    model.onDidChangeContent(() => emit('update:content', model!.getValue()))
    models.set(path, model)
  }
  return model
}

function applyRunDecorations() {
  if (!editor) return
  const actions = props.runActions ?? []
  lineToAction = new Map(actions.map((a) => [a.line, a]))
  const decos: Monaco.editor.IModelDeltaDecoration[] = actions.map((a) => ({
    range: new monaco.Range(a.line, 1, a.line, 1),
    options: {
      glyphMarginClassName: 'run-glyph',
      glyphMarginHoverMessage: { value: a.tooltip ?? `Run \`${a.command}\`` },
      stickiness: monaco.editor.TrackedRangeStickiness.NeverGrowsWhenTypingAtEdges,
    },
  }))
  if (!runDecorations) runDecorations = editor.createDecorationsCollection(decos)
  else runDecorations.set(decos)
}

onMounted(() => {
  if (!containerRef.value) return
  editor = monaco.editor.create(containerRef.value, {
    theme: 'vs-dark',
    automaticLayout: true,
    glyphMargin: true,
    minimap: { enabled: true },
    fontSize: 14,
    lineNumbers: 'on',
    roundedSelection: false,
    scrollBeyondLastLine: false,
    wordWrap: 'off',
    tabSize: 2,
  })
  editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => emit('save'))

  // Click a gutter glyph -> run that action.
  editor.onMouseDown((e) => {
    if (e.target.type !== monaco.editor.MouseTargetType.GUTTER_GLYPH_MARGIN) return
    const line = e.target.position?.lineNumber
    const action = line ? lineToAction.get(line) : undefined
    if (action) emit('run-action', action)
  })

  if (props.activePath) {
    editor.setModel(modelFor(props.activePath, props.content))
  }
  applyRunDecorations()

  resizeObserver = new ResizeObserver(() => editor?.layout())
  resizeObserver.observe(containerRef.value)
})

watch(() => props.activePath, (path) => {
  if (!editor) return
  editor.setModel(path ? modelFor(path, props.content) : null)
  // Drop the previous file's glyphs immediately. The new file's actions arrive
  // via the runActions signature watch below (the parent recomputes on switch),
  // which applies them authoritatively — avoiding a stale-glyph flash here.
  runDecorations?.clear()
})

// Re-render glyphs whenever the actions for the current file change. A stable
// signature avoids churn on every keystroke.
watch(
  () => (props.runActions ?? []).map((a) => `${a.line}:${a.id}`).join('|'),
  applyRunDecorations,
)

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
  runDecorations?.clear()
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
