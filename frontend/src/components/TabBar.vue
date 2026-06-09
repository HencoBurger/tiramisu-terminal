<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useTabs } from '../composables/useTabs'
import type { TabType } from '../types/session'
import TabItem from './TabItem.vue'

const { tabs, activeTabId, setActiveTab, removeTab, renameTab, addTab } = useTabs()

const emit = defineEmits<{
  tabContextMenu: [tabId: string, event: MouseEvent]
}>()

const showMenu = ref(false)
const menuStyle = ref({ top: '0px', left: '0px' })

function handleClose(id: string) {
  removeTab(id)
}

function toggleMenu(e: MouseEvent) {
  if (showMenu.value) {
    showMenu.value = false
    return
  }
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
  menuStyle.value = {
    top: rect.bottom + 2 + 'px',
    left: rect.left + 'px',
  }
  showMenu.value = true
}

function handleNewTab(type: TabType) {
  showMenu.value = false
  addTab('', '', type)
}

function onDocClick() {
  showMenu.value = false
}

onMounted(() => document.addEventListener('click', onDocClick))
onUnmounted(() => document.removeEventListener('click', onDocClick))
</script>

<template>
  <div class="tab-bar flex items-end px-1 gap-0.5 overflow-x-auto" style="--wails-draggable: drag">
    <TabItem
      v-for="tab in tabs"
      :key="tab.id"
      :tab="tab"
      :is-active="tab.id === activeTabId"
      class="group"
      @select="setActiveTab(tab.id)"
      @close="handleClose(tab.id)"
      @rename="(name) => renameTab(tab.id, name)"
      @contextmenu="(e) => emit('tabContextMenu', tab.id, e)"
    />
    <button
      class="btn btn-ghost btn-sm px-2 min-h-0 h-7 text-base-content/60 hover:text-base-content"
      style="--wails-draggable: no-drag"
      @click.stop="toggleMenu"
    >
      +
    </button>
  </div>

  <Teleport to="body">
    <ul
      v-if="showMenu"
      class="fixed z-[9999] menu p-1 shadow-lg bg-base-200 rounded-box w-36"
      :style="menuStyle"
      @click.stop
    >
      <li><a @click="handleNewTab('chat')">Chat</a></li>
      <li><a @click="handleNewTab('terminal')">Terminal</a></li>
    </ul>
  </Teleport>
</template>
