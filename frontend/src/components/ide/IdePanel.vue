<script lang="ts" setup>
import { ref, computed, watch, onMounted, nextTick } from 'vue'
import { ReadFile, WriteFile, ListDirectory } from '../../../wailsjs/go/main/App'
import { useTabs } from '../../composables/useTabs'
import { useConfig } from '../../composables/useConfig'
import type { TabState } from '../../types/session'
import WorkDirPicker from '../WorkDirPicker.vue'
import FileTree from './FileTree.vue'
import EditorPane from './EditorPane.vue'
import MarkdownPreview from './MarkdownPreview.vue'
import { guessLanguage } from './language'
import { getFileIconUrl } from './fileIcons'
import { computeRunActions, type RunAction } from './run'
import { resetDetectCache } from './run/detect'

interface FileEntry {
  name: string
  path: string
  isDir: boolean
}

interface OpenFile {
  path: string
  name: string
  modified: boolean
}

const props = defineProps<{
  tab: TabState
}>()

const { activeTabId, setTabWorkDir, setTabOpenFiles, addTab, setTabInitialCommand } = useTabs()
const { maybeSetDefaultWorkDir } = useConfig()

const rootEntries = ref<FileEntry[]>([])
const openFiles = ref<OpenFile[]>([])
const activePath = ref('')
const fileContents = ref<Record<string, string>>({})
const originalContents = ref<Record<string, string>>({})
const loading = ref(false)
const showWorkDirPicker = ref(false)
const editorRef = ref<InstanceType<typeof EditorPane>>()

const activeContent = computed(() => fileContents.value[activePath.value] ?? '')
const activeLanguage = computed(() => guessLanguage(activePath.value))

// Markdown view mode (only relevant for .md files): code / split / preview.
const mdView = ref<'code' | 'split' | 'preview'>('preview')
const isMarkdown = computed(() => activeLanguage.value === 'markdown')
const showEditor = computed(() => !isMarkdown.value || mdView.value !== 'preview')
const showPreview = computed(() => isMarkdown.value && mdView.value !== 'code')

// Gutter run actions for the active file (package.json scripts in v1; the run/
// registry makes any language a one-file add-on).
const runActions = ref<RunAction[]>([])
let runSeq = 0
let runDebounce: ReturnType<typeof setTimeout> | null = null

async function recomputeRunActions() {
  const path = activePath.value
  if (!path) {
    runActions.value = []
    return
  }
  const seq = ++runSeq
  resetDetectCache()
  const actions = await computeRunActions({
    path,
    fileName: path.split(/[\\/]/).pop() ?? path,
    dir: path.replace(/[\\/][^\\/]+$/, '') || props.tab.workDir,
    content: activeContent.value,
    language: activeLanguage.value,
    workDir: props.tab.workDir,
    listDir: ListDirectory,
  })
  if (seq === runSeq) runActions.value = actions // ignore stale async results
}

// Recompute immediately on file switch; debounce on content edits.
watch(activePath, recomputeRunActions, { immediate: true })
watch(activeContent, () => {
  if (runDebounce) clearTimeout(runDebounce)
  runDebounce = setTimeout(recomputeRunActions, 300)
})

function handleRunAction(action: RunAction) {
  const tab = addTab(action.cwd, '▶ ' + action.label, 'terminal')
  setTabInitialCommand(tab.id, action.command)
}

async function loadRoot() {
  if (!props.tab.workDir) return
  try {
    rootEntries.value = await ListDirectory(props.tab.workDir)
  } catch (e) {
    console.error('Failed to list working directory:', e)
  }
}

async function openFile(path: string) {
  if (openFiles.value.some((f) => f.path === path)) {
    activePath.value = path
    return
  }

  loading.value = true
  try {
    const content = await ReadFile(path)
    fileContents.value[path] = content
    originalContents.value[path] = content
    const name = path.split(/[\\/]/).pop() ?? path
    openFiles.value.push({ path, name, modified: false })
    activePath.value = path
  } catch (e: any) {
    console.error('Failed to open file:', e)
  } finally {
    loading.value = false
  }
}

function updateContent(value: string) {
  fileContents.value[activePath.value] = value
  const f = openFiles.value.find((f) => f.path === activePath.value)
  if (f) f.modified = value !== originalContents.value[activePath.value]
}

async function saveFile() {
  const path = activePath.value
  if (!path) return
  const content = fileContents.value[path]
  try {
    await WriteFile(path, content)
    originalContents.value[path] = content
    const f = openFiles.value.find((f) => f.path === path)
    if (f) f.modified = false
  } catch (e) {
    console.error('Failed to save file:', e)
  }
}

function closeFile(path: string) {
  const idx = openFiles.value.findIndex((f) => f.path === path)
  if (idx === -1) return
  openFiles.value.splice(idx, 1)
  delete fileContents.value[path]
  delete originalContents.value[path]

  if (activePath.value === path) {
    activePath.value = openFiles.value[Math.min(idx, openFiles.value.length - 1)]?.path ?? ''
  }
  // Dispose the closed file's model only after the editor has switched models
  // (next tick), so it never briefly holds a disposed model.
  nextTick(() => editorRef.value?.disposeModel(path))
}

function handleWorkDirSelected(dir: string) {
  setTabWorkDir(props.tab.id, dir)
  maybeSetDefaultWorkDir(dir)
}

watch(() => props.tab.workDir, (dir) => {
  if (dir) loadRoot()
})

// Re-measure Monaco when this tab becomes active (it's kept mounted under v-show).
watch(() => activeTabId.value, (id) => {
  if (id === props.tab.id) nextTick(() => editorRef.value?.layout())
})

// Re-measure Monaco when the markdown view toggles change its width/visibility.
watch([mdView, activePath], () => {
  if (showEditor.value) nextTick(() => editorRef.value?.layout())
})

// Persist the open-file list + active file back onto the tab so it survives restarts.
// Keyed on paths + active path only (not dirty flags), so it fires only on real changes.
watch(
  () => openFiles.value.map((f) => f.path).join('\n') + '\n' + activePath.value,
  () => setTabOpenFiles(props.tab.id, openFiles.value.map((f) => f.path), activePath.value)
)

// Reopen the files that were open in this tab last session, from disk.
async function restoreOpenFiles() {
  const saved = props.tab.openFiles ?? []
  if (!saved.length) return
  for (const path of saved) {
    await openFile(path) // gracefully skips files that no longer exist
  }
  const active = props.tab.activeFile
  if (active && openFiles.value.some((f) => f.path === active)) {
    activePath.value = active
  }
}

onMounted(async () => {
  if (props.tab.workDir) {
    loadRoot()
    await restoreOpenFiles()
  } else {
    showWorkDirPicker.value = true
  }
})
</script>

<template>
  <div class="h-full flex flex-col">
    <!-- Empty state: no working directory chosen yet -->
    <div v-if="!tab.workDir" class="flex items-center justify-center h-full text-base-content/30">
      <div class="text-center">
        <p class="text-lg mb-2">Editor</p>
        <p class="text-sm">
          Select a working directory to get started
          <button class="btn btn-sm btn-outline ml-2" @click="showWorkDirPicker = true">
            Choose Directory
          </button>
        </p>
      </div>
    </div>

    <div v-else class="flex flex-1 min-h-0">
      <!-- Sidebar: file tree -->
      <div class="w-60 min-w-48 shrink-0 bg-base-200 border-r border-base-300 flex flex-col overflow-hidden">
        <div class="px-3 py-1.5 text-xs uppercase tracking-wide opacity-60 shrink-0">
          IDE
        </div>
        <div class="flex-1 overflow-auto">
          <FileTree :entries="rootEntries" @select-file="openFile" />
        </div>
      </div>

      <!-- Editor column -->
      <div class="flex flex-col flex-1 min-w-0">
        <!-- Open-file tab strip -->
        <div v-if="openFiles.length" class="flex items-stretch bg-base-200 border-b border-base-300 overflow-x-auto shrink-0">
          <div
            v-for="f in openFiles"
            :key="f.path"
            class="group flex items-center gap-1.5 px-3 py-1 text-sm cursor-pointer border-r border-base-300 whitespace-nowrap"
            :class="f.path === activePath ? 'bg-base-100' : 'opacity-70 hover:opacity-100'"
            @click="activePath = f.path"
          >
            <img :src="getFileIconUrl(f.name)" class="w-4 h-4 shrink-0" alt="" />
            <span class="truncate max-w-[12rem]">{{ f.name }}</span>
            <span v-if="f.modified" class="w-1.5 h-1.5 rounded-full bg-warning shrink-0" />
            <button
              class="opacity-0 group-hover:opacity-60 hover:!opacity-100 leading-none"
              @click.stop="closeFile(f.path)"
            >×</button>
          </div>
        </div>

        <!-- Markdown view toggle (only for .md files) -->
        <div
          v-if="!loading && activePath && isMarkdown"
          class="flex items-center px-2 py-1 bg-base-200 border-b border-base-300 shrink-0"
        >
          <div class="join">
            <button class="btn btn-xs join-item" :class="mdView === 'code' ? 'btn-active' : ''" @click="mdView = 'code'">Code</button>
            <button class="btn btn-xs join-item" :class="mdView === 'split' ? 'btn-active' : ''" @click="mdView = 'split'">Split</button>
            <button class="btn btn-xs join-item" :class="mdView === 'preview' ? 'btn-active' : ''" @click="mdView = 'preview'">Preview</button>
          </div>
        </div>

        <div v-if="loading" class="flex-1 flex items-center justify-center bg-base-100">
          <span class="loading loading-spinner loading-lg" />
        </div>

        <!-- Editor + markdown preview area (editor stays mounted to keep models) -->
        <div v-show="!loading && activePath" class="flex flex-1 min-h-0">
          <div
            v-show="showEditor"
            class="min-h-0"
            :class="showPreview ? 'w-1/2 shrink-0' : 'flex-1'"
          >
            <EditorPane
              ref="editorRef"
              class="h-full"
              :active-path="activePath"
              :content="activeContent"
              :run-actions="runActions"
              @update:content="updateContent"
              @save="saveFile"
              @run-action="handleRunAction"
            />
          </div>
          <MarkdownPreview
            v-if="showPreview"
            :content="activeContent"
            class="min-h-0 flex-1 overflow-auto bg-base-100"
            :class="showEditor ? 'border-l border-base-300' : ''"
          />
        </div>

        <div
          v-if="!loading && !activePath"
          class="flex-1 flex items-center justify-center bg-base-100 text-base-content/30 text-sm"
        >
          Open a file to start editing
        </div>

        <!-- Status line -->
        <div class="flex items-center gap-3 px-3 h-6 text-xs bg-base-300/50 shrink-0">
          <span class="truncate opacity-70">{{ activePath || tab.workDir }}</span>
          <span v-if="activePath" class="opacity-50 ml-auto">{{ activeLanguage }}</span>
        </div>
      </div>
    </div>

    <WorkDirPicker
      :open="showWorkDirPicker"
      @update:open="showWorkDirPicker = $event"
      @select="handleWorkDirSelected"
    />
  </div>
</template>
