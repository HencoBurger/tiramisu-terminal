<script setup lang="ts">
import { ref } from 'vue'
import { OpenDirectoryDialog } from '../../wailsjs/go/main/App'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  select: [dir: string]
}>()

const selectedDir = ref('')

async function browseDir() {
  try {
    const dir = await OpenDirectoryDialog()
    if (dir) {
      selectedDir.value = dir
    }
  } catch (e) {
    console.error('Failed to open directory dialog:', e)
  }
}

function confirm() {
  if (selectedDir.value) {
    emit('select', selectedDir.value)
    emit('update:open', false)
    selectedDir.value = ''
  }
}

function close() {
  emit('update:open', false)
  selectedDir.value = ''
}
</script>

<template>
  <dialog class="modal" :class="{ 'modal-open': open }">
    <div class="modal-box">
      <h3 class="text-lg font-bold mb-4">Select Working Directory</h3>
      <div class="flex gap-2">
        <input
          v-model="selectedDir"
          type="text"
          placeholder="Path to project directory..."
          class="input input-bordered flex-1"
        />
        <button class="btn btn-outline" @click="browseDir">Browse</button>
      </div>
      <div class="modal-action">
        <button class="btn btn-ghost" @click="close">Cancel</button>
        <button class="btn btn-primary" :disabled="!selectedDir" @click="confirm">Open</button>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop" @click="close">
      <button>close</button>
    </form>
  </dialog>
</template>
