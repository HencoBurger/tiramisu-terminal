<script setup lang="ts">
import { ref, watch } from 'vue'
import { ListWindowSessions, DeleteWindowSession } from '../../wailsjs/go/main/App'
import type { WindowSessionSummary } from '../types/session'

const props = defineProps<{
  open: boolean
  mandatory: boolean
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  select: [id: string]
  create: [name: string]
}>()

const sessions = ref<WindowSessionSummary[]>([])
const loading = ref(false)
const newName = ref('')
const confirmDeleteId = ref<string | null>(null)

watch(() => props.open, async (isOpen) => {
  if (isOpen) {
    confirmDeleteId.value = null
    await loadSessions()
  }
})

async function loadSessions() {
  loading.value = true
  try {
    sessions.value = await ListWindowSessions() || []
  } catch (e) {
    console.error('Failed to list window sessions:', e)
  } finally {
    loading.value = false
  }
}

function handleCreate() {
  const name = newName.value.trim()
  if (!name) return
  newName.value = ''
  emit('create', name)
  emit('update:open', false)
}

function handleSelect(id: string) {
  emit('select', id)
  emit('update:open', false)
}

async function handleDelete(id: string) {
  try {
    await DeleteWindowSession(id)
    sessions.value = sessions.value.filter(s => s.id !== id)
  } catch (e) {
    console.error('Failed to delete session:', e)
  }
  confirmDeleteId.value = null
}

function close() {
  if (props.mandatory) return
  emit('update:open', false)
}

function relativeTime(ts: number): string {
  const now = Date.now()
  const diff = now - ts * 1000
  const seconds = Math.floor(diff / 1000)
  if (seconds < 60) return 'just now'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  if (days < 30) return `${days}d ago`
  return new Date(ts * 1000).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}
</script>

<template>
  <dialog class="modal" :class="{ 'modal-open': open }">
    <div class="modal-box max-w-lg">
      <h3 class="text-lg font-bold mb-4">Select Session</h3>

      <!-- New session -->
      <div class="flex gap-2 mb-4">
        <input
          v-model="newName"
          type="text"
          placeholder="New session name..."
          class="input input-bordered input-sm flex-1"
          @keydown.enter="handleCreate"
        />
        <button
          class="btn btn-primary btn-sm"
          :disabled="!newName.trim()"
          @click="handleCreate"
        >
          Create
        </button>
      </div>

      <div v-if="sessions.length > 0" class="divider text-xs text-base-content/40">or open existing</div>

      <!-- Loading -->
      <div v-if="loading" class="flex justify-center py-4">
        <span class="loading loading-spinner loading-md"></span>
      </div>

      <!-- Session list -->
      <div v-else-if="sessions.length > 0" class="overflow-y-auto max-h-64">
        <table class="table table-sm">
          <thead>
            <tr>
              <th>Name</th>
              <th>Tabs</th>
              <th>Last Opened</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="session in sessions" :key="session.id" class="hover">
              <td class="font-medium">{{ session.name }}</td>
              <td>{{ session.tabCount }}</td>
              <td class="text-xs" :title="new Date(session.lastOpenedAt * 1000).toLocaleString()">
                {{ relativeTime(session.lastOpenedAt) }}
              </td>
              <td class="flex gap-1 justify-end">
                <button class="btn btn-sm btn-outline" @click="handleSelect(session.id)">
                  Open
                </button>
                <button
                  v-if="confirmDeleteId !== session.id"
                  class="btn btn-sm btn-ghost text-error"
                  @click="confirmDeleteId = session.id"
                >
                  Delete
                </button>
                <template v-else>
                  <button class="btn btn-sm btn-error" @click="handleDelete(session.id)">
                    Confirm
                  </button>
                  <button class="btn btn-sm btn-ghost" @click="confirmDeleteId = null">
                    Cancel
                  </button>
                </template>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="!mandatory" class="modal-action">
        <button class="btn btn-ghost" @click="close">Cancel</button>
      </div>
    </div>
    <form v-if="!mandatory" method="dialog" class="modal-backdrop" @click="close">
      <button>close</button>
    </form>
  </dialog>
</template>
