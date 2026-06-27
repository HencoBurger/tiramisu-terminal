<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useTabs } from '../composables/useTabs'
import type { TabType } from '../types/session'
import TabItem from './TabItem.vue'

const { tabs, activeTabId, setActiveTab, renameTab, addTab, moveTab } = useTabs()

const emit = defineEmits<{
  tabContextMenu: [tabId: string, event: MouseEvent]
  tabClose: [tabId: string]
}>()

const showMenu = ref(false)
const menuStyle = ref({ top: '0px', left: '0px' })

// Tab drag-and-drop reordering (pointer events; HTML5 DnD is unreliable in webkit2gtk)
const barRef = ref<HTMLElement>()
const dragState = ref<{ tabId: string; startX: number; active: boolean } | null>(null)

function onTabDragStart(tabId: string, e: PointerEvent) {
  if (e.button !== 0) return
  dragState.value = { tabId, startX: e.clientX, active: false }
  document.addEventListener('pointermove', onDragMove)
  document.addEventListener('pointerup', onDragEnd)
  document.addEventListener('pointercancel', onDragEnd)
}

function onDragMove(e: PointerEvent) {
  const st = dragState.value
  if (!st) return
  if (!st.active) {
    if (Math.abs(e.clientX - st.startX) <= 5) return
    st.active = true
  }
  const els = Array.from(barRef.value?.querySelectorAll('.tab-item') ?? []) as HTMLElement[]
  const from = tabs.value.findIndex(t => t.id === st.tabId)
  if (from === -1 || els.length !== tabs.value.length) return
  for (let i = 0; i < els.length; i++) {
    if (i === from) continue
    const r = els[i].getBoundingClientRect()
    const mid = r.left + r.width / 2
    if ((i < from && e.clientX < mid) || (i > from && e.clientX > mid)) {
      moveTab(st.tabId, i)
      break
    }
  }
}

function onDragEnd() {
  const st = dragState.value
  if (st?.active) setActiveTab(st.tabId)
  dragState.value = null
  document.removeEventListener('pointermove', onDragMove)
  document.removeEventListener('pointerup', onDragEnd)
  document.removeEventListener('pointercancel', onDragEnd)
}

function handleClose(id: string) {
  emit('tabClose', id)
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
onUnmounted(() => {
  document.removeEventListener('click', onDocClick)
  onDragEnd()
})
</script>

<template>
  <div ref="barRef" class="tab-bar flex items-end px-1 gap-0.5 overflow-x-auto" style="--wails-draggable: drag">
    <TabItem
      v-for="tab in tabs"
      :key="tab.id"
      :tab="tab"
      :is-active="tab.id === activeTabId"
      :is-dragging="dragState?.active && dragState.tabId === tab.id"
      class="group"
      @select="setActiveTab(tab.id)"
      @close="handleClose(tab.id)"
      @rename="(name) => renameTab(tab.id, name)"
      @contextmenu="(e) => emit('tabContextMenu', tab.id, e)"
      @dragstart="(e) => onTabDragStart(tab.id, e)"
    />
    <button
      class="btn btn-ghost btn-sm min-h-0 h-8 w-9 shrink-0 sticky right-0 bg-base-300 text-lg leading-none text-base-content/60 hover:text-base-content"
      style="--wails-draggable: no-drag"
      title="New tab (Ctrl+T)"
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
      <li><a @click="handleNewTab('ide')">Editor</a></li>
    </ul>
  </Teleport>
</template>
