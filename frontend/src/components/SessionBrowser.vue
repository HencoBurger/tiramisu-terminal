<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { StoredSession } from '../types/session'
import { ListClaudeSessions } from '../../wailsjs/go/main/App'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  resume: [session: StoredSession]
}>()

const sessions = ref<StoredSession[]>([])
const searchQuery = ref('')
const loading = ref(false)

const filtered = computed(() => {
  const q = searchQuery.value.toLowerCase()
  if (!q) return sessions.value
  return sessions.value.filter(s =>
    s.firstPrompt.toLowerCase().includes(q) ||
    s.projectDir.toLowerCase().includes(q) ||
    s.sessionId.toLowerCase().includes(q)
  )
})

watch(() => props.open, async (isOpen) => {
  if (isOpen) {
    loading.value = true
    try {
      sessions.value = await ListClaudeSessions() || []
    } catch (e) {
      console.error('Failed to list sessions:', e)
    } finally {
      loading.value = false
    }
  }
})

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

function shortPath(dir: string): string {
  const parts = dir.replace(/\/$/, '').split('/')
  return parts.length > 2 ? '…/' + parts.slice(-2).join('/') : dir
}

function handleResume(session: StoredSession) {
  emit('resume', session)
  emit('update:open', false)
}

function close() {
  emit('update:open', false)
}
</script>

<template>
  <dialog class="modal" :class="{ 'modal-open': open }">
    <div class="modal-box w-11/12 max-w-3xl">
      <h3 class="text-lg font-bold mb-4">Browse Sessions</h3>

      <input
        v-model="searchQuery"
        type="text"
        placeholder="Search sessions..."
        class="input input-bordered w-full mb-4"
      />

      <div v-if="loading" class="flex justify-center py-8">
        <span class="loading loading-spinner loading-lg"></span>
      </div>

      <div v-else-if="filtered.length === 0" class="text-center py-8 text-base-content/50">
        No sessions found
      </div>

      <div v-else class="overflow-x-auto max-h-96">
        <table class="table table-sm">
          <thead>
            <tr>
              <th>First Prompt</th>
              <th>Project</th>
              <th>Messages</th>
              <th>Last Modified</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="session in filtered" :key="session.sessionId" class="hover">
              <td class="max-w-xs truncate" :title="session.firstPrompt">
                {{ session.firstPrompt || '(empty)' }}
              </td>
              <td class="text-xs font-mono max-w-32 truncate" :title="session.projectDir">
                {{ shortPath(session.projectDir) }}
              </td>
              <td>{{ session.messageCount }}</td>
              <td class="text-xs" :title="new Date(session.lastModified * 1000).toLocaleString()">
                {{ relativeTime(session.lastModified) }}
              </td>
              <td>
                <button class="btn btn-sm btn-outline" @click="handleResume(session)">
                  Resume
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="modal-action">
        <button class="btn btn-ghost" @click="close">Close</button>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop" @click="close">
      <button>close</button>
    </form>
  </dialog>
</template>
