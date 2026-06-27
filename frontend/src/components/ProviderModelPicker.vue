<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { TabState } from '../types/session'
import type { main } from '../../wailsjs/go/models'
import { useTabs } from '../composables/useTabs'
import { useConfig } from '../composables/useConfig'
import { useAgent } from '../composables/useAgent'

const props = defineProps<{ tab: TabState }>()
const { setTabProvider, setTabModel, setTabWorkerModel } = useTabs()
const { globalConfig } = useConfig()
const { agentStop, listProviderModels } = useAgent()

const claudeModels = [
  { id: '', name: 'Default' },
  { id: 'sonnet', name: 'Sonnet' },
  { id: 'opus', name: 'Opus' },
  { id: 'haiku', name: 'Haiku' },
]

const provider = computed(() => props.tab.provider || 'claude')
const providers = computed(() =>
  globalConfig.value.enabledProviders?.length
    ? globalConfig.value.enabledProviders
    : ['claude', 'ollama', 'openrouter'],
)

const nativeModels = ref<main.ModelInfo[]>([])
const loading = ref(false)
const loadError = ref('')

async function loadModels(p: string) {
  if (p === 'claude') {
    nativeModels.value = []
    loadError.value = ''
    return
  }
  loading.value = true
  loadError.value = ''
  try {
    nativeModels.value = await listProviderModels(p)
  } catch (e: any) {
    loadError.value = e?.message || String(e)
    nativeModels.value = []
  } finally {
    loading.value = false
  }
}

watch(provider, (p) => loadModels(p), { immediate: true })

function label(p: string) {
  return p.charAt(0).toUpperCase() + p.slice(1)
}

function onProviderChange(e: Event) {
  const p = (e.target as HTMLSelectElement).value
  // Switching provider resets the native conversation — context can't carry across.
  agentStop(props.tab.id).catch(() => {})
  setTabProvider(props.tab.id, p === 'claude' ? '' : p)
  setTabModel(props.tab.id, '') // clear; pick a model for the new provider
  setTabWorkerModel(props.tab.id, '')
}

function onModelChange(e: Event) {
  setTabModel(props.tab.id, (e.target as HTMLSelectElement).value)
}

function onWorkerChange(e: Event) {
  setTabWorkerModel(props.tab.id, (e.target as HTMLSelectElement).value)
}
</script>

<template>
  <div class="flex items-center gap-1">
    <select
      :value="provider"
      class="select select-ghost select-xs text-xs h-5 min-h-0 pl-1 pr-5"
      title="Chat provider"
      @change="onProviderChange"
    >
      <option v-for="p in providers" :key="p" :value="p">{{ label(p) }}</option>
    </select>

    <!-- Claude: static model aliases -->
    <select
      v-if="provider === 'claude'"
      :value="tab.model"
      class="select select-ghost select-xs text-xs h-5 min-h-0 pl-1 pr-5"
      title="Model"
      @change="onModelChange"
    >
      <option v-for="m in claudeModels" :key="m.id" :value="m.id">{{ m.name }}</option>
    </select>

    <!-- Native: models fetched from the provider -->
    <template v-else>
      <span v-if="loading" class="loading loading-spinner loading-xs" />
      <select
        v-else-if="!loadError"
        :value="tab.model"
        class="select select-ghost select-xs text-xs h-5 min-h-0 pl-1 pr-5 max-w-[14rem]"
        title="Model"
        @change="onModelChange"
      >
        <option value="">Select model…</option>
        <option v-for="m in nativeModels" :key="m.id" :value="m.id">{{ m.name }}</option>
      </select>
      <span v-else class="text-error text-xs cursor-help" :title="loadError">⚠ models unavailable</span>

      <!-- Worker (sub-agent) model for the delegate tool -->
      <template v-if="!loading && !loadError">
        <span class="text-xs opacity-40">worker</span>
        <select
          :value="tab.workerModel || ''"
          class="select select-ghost select-xs text-xs h-5 min-h-0 pl-1 pr-5 max-w-[12rem]"
          title="Worker (sub-agent) model used by the delegate tool"
          @change="onWorkerChange"
        >
          <option value="">(same as main)</option>
          <option v-for="m in nativeModels" :key="m.id" :value="m.id">{{ m.name }}</option>
        </select>
      </template>
    </template>
  </div>
</template>
