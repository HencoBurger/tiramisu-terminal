<script lang="ts" setup>
import { ref } from 'vue'
import { ListDirectory } from '../../../wailsjs/go/main/App'
import { getFileIconUrl, getFolderIconUrl } from './fileIcons'

interface FileEntry {
  name: string
  path: string
  isDir: boolean
}

defineProps<{
  entries: FileEntry[]
  depth?: number
}>()

const emit = defineEmits<{
  selectFile: [path: string]
}>()

const expanded = ref<Record<string, boolean>>({})
const children = ref<Record<string, FileEntry[]>>({})

async function toggle(entry: FileEntry) {
  if (!entry.isDir) {
    emit('selectFile', entry.path)
    return
  }

  if (expanded.value[entry.path]) {
    expanded.value[entry.path] = false
    return
  }

  if (!children.value[entry.path]) {
    try {
      children.value[entry.path] = await ListDirectory(entry.path)
    } catch (e) {
      console.error('Failed to list directory:', e)
      return
    }
  }
  expanded.value[entry.path] = true
}
</script>

<template>
  <ul class="menu menu-xs p-0 w-full flex-nowrap">
    <li v-for="entry in entries" :key="entry.path">
      <a
        class="flex items-center gap-1 rounded-none py-0.5"
        :style="{ paddingLeft: (depth ?? 0) * 16 + 8 + 'px' }"
        @click="toggle(entry)"
      >
        <span v-if="entry.isDir" class="text-[10px] w-3 text-center shrink-0 opacity-60">
          {{ expanded[entry.path] ? '▾' : '▸' }}
        </span>
        <span v-else class="w-3 shrink-0"></span>
        <img
          :src="entry.isDir ? getFolderIconUrl(entry.name, !!expanded[entry.path]) : getFileIconUrl(entry.name)"
          class="w-4 h-4 shrink-0"
          alt=""
        />
        <span class="truncate">{{ entry.name }}</span>
      </a>
      <FileTree
        v-if="entry.isDir && expanded[entry.path] && children[entry.path]"
        :entries="children[entry.path]"
        :depth="(depth ?? 0) + 1"
        @select-file="(p: string) => emit('selectFile', p)"
      />
    </li>
  </ul>
</template>
