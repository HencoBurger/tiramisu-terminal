<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'

const props = withDefaults(defineProps<{
  open: boolean
  title: string
  message: string
  confirmLabel?: string
  danger?: boolean
}>(), {
  confirmLabel: 'Confirm',
  danger: false,
})

const emit = defineEmits<{
  confirm: []
  cancel: []
}>()

const confirmBtn = ref<HTMLButtonElement | null>(null)

watch(() => props.open, (open) => {
  if (open) {
    nextTick(() => confirmBtn.value?.focus())
  }
})
</script>

<template>
  <dialog
    class="modal"
    :class="{ 'modal-open': open }"
    @keydown.escape.prevent="emit('cancel')"
  >
    <div class="modal-box">
      <h3 class="text-lg font-bold mb-2">{{ title }}</h3>
      <p class="text-base-content/80">{{ message }}</p>
      <div class="modal-action">
        <button class="btn btn-ghost" @click="emit('cancel')">Cancel</button>
        <button
          ref="confirmBtn"
          class="btn"
          :class="danger ? 'btn-error' : 'btn-primary'"
          @click="emit('confirm')"
        >
          {{ confirmLabel }}
        </button>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop" @click="emit('cancel')">
      <button>close</button>
    </form>
  </dialog>
</template>
