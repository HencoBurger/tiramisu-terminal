<script setup lang="ts">
import { ref, watch } from 'vue'
import { useConfig } from '../composables/useConfig'
import { useSound } from '../composables/useSound'
import { useAgent } from '../composables/useAgent'
import type { Profile } from '../types/session'
import { OpenDirectoryDialog } from '../../wailsjs/go/main/App'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
}>()

const { globalConfig, windowSession, effectiveConfig, saveGlobalConfig, saveWindowSession } = useConfig()
const { SOUNDS, play } = useSound()
const { setProviderKey, hasProviderKey, deleteProviderKey } = useAgent()

// Providers
const localOllamaBaseURL = ref('')
const openRouterKeyInput = ref('')
const hasOpenRouterKey = ref(false)
const providerError = ref('')
const localDisableThinking = ref(false)
const localCustomInstructions = ref('')

// Global defaults
const localSound = ref('')
const localTheme = ref('')
const localPermMode = ref('')
const localProfiles = ref<Profile[]>([])

// Session overrides
const overrideTheme = ref(false)
const overrideSound = ref(false)
const overridePermMode = ref(false)
const localSessionTheme = ref('')
const localSessionSound = ref('')
const localSessionPermMode = ref('')

const themes = ['dark', 'light', 'dracula', 'night', 'dim', 'sunset', 'business', 'coffee']

const permissionModes = [
  { value: 'default', label: 'Default', desc: 'Claude asks for permission (may fail in non-interactive mode)' },
  { value: 'acceptEdits', label: 'Accept Edits', desc: 'Auto-approve file read/write/edit, prompt for bash commands' },
  { value: 'bypassPermissions', label: 'Bypass All', desc: 'Skip all permission checks (use with caution)' },
]

// New profile form
const newProfileName = ref('')
const newProfileHomeDir = ref('')

watch(() => props.open, (isOpen) => {
  if (isOpen) {
    // Load global values
    localSound.value = globalConfig.value.defaultSound
    localTheme.value = globalConfig.value.theme
    localPermMode.value = globalConfig.value.permissionMode || 'default'
    localProfiles.value = JSON.parse(JSON.stringify(globalConfig.value.profiles || []))

    // Load session override state
    const ws = windowSession.value
    overrideTheme.value = !!(ws?.themeOverride)
    overrideSound.value = !!(ws?.soundOverride)
    overridePermMode.value = !!(ws?.permModeOverride)
    localSessionTheme.value = ws?.themeOverride || localTheme.value
    localSessionSound.value = ws?.soundOverride || localSound.value
    localSessionPermMode.value = ws?.permModeOverride || localPermMode.value

    // Providers
    localOllamaBaseURL.value = globalConfig.value.ollamaBaseURL || 'http://localhost:11434'
    localDisableThinking.value = !!globalConfig.value.disableThinking
    localCustomInstructions.value = globalConfig.value.customInstructions || ''
    openRouterKeyInput.value = ''
    providerError.value = ''
    hasProviderKey('openrouter').then((v) => (hasOpenRouterKey.value = v)).catch(() => {})
  }
})

async function saveOpenRouterKey() {
  const key = openRouterKeyInput.value.trim()
  if (!key) return
  providerError.value = ''
  try {
    await setProviderKey('openrouter', key)
    hasOpenRouterKey.value = true
    openRouterKeyInput.value = ''
  } catch (e: any) {
    providerError.value = e?.message || String(e)
  }
}

async function clearOpenRouterKey() {
  try {
    await deleteProviderKey('openrouter')
    hasOpenRouterKey.value = false
  } catch {
    // ignore
  }
}

const soundError = ref('')
async function previewSound(name: string) {
  soundError.value = ''
  const err = await play(name)
  if (err) soundError.value = `${err.name}: ${err.message}`
}

async function browseHomeDir() {
  const dir = await OpenDirectoryDialog()
  if (dir) {
    newProfileHomeDir.value = dir
  }
}

function addProfile() {
  const name = newProfileName.value.trim()
  const homeDir = newProfileHomeDir.value.trim()
  if (!name || !homeDir) return

  localProfiles.value.push({
    id: `profile-${Date.now()}`,
    name,
    homeDir,
  })
  newProfileName.value = ''
  newProfileHomeDir.value = ''
}

function removeProfile(id: string) {
  localProfiles.value = localProfiles.value.filter(p => p.id !== id)
}

async function save() {
  // Save global config
  await saveGlobalConfig({
    defaultSound: localSound.value,
    theme: localTheme.value,
    permissionMode: localPermMode.value,
    profiles: localProfiles.value,
    ollamaBaseURL: localOllamaBaseURL.value.trim() || 'http://localhost:11434',
    enabledProviders: globalConfig.value.enabledProviders,
    defaultModels: globalConfig.value.defaultModels,
    disableThinking: localDisableThinking.value,
    customInstructions: localCustomInstructions.value,
  })

  // Save session overrides
  if (windowSession.value) {
    windowSession.value.themeOverride = overrideTheme.value ? localSessionTheme.value : ''
    windowSession.value.soundOverride = overrideSound.value ? localSessionSound.value : ''
    windowSession.value.permModeOverride = overridePermMode.value ? localSessionPermMode.value : ''
    await saveWindowSession()
  }

  // Apply effective theme
  document.documentElement.setAttribute('data-theme', effectiveConfig.value.theme)

  emit('update:open', false)
}

function close() {
  emit('update:open', false)
}
</script>

<template>
  <dialog class="modal" :class="{ 'modal-open': open }">
    <div class="modal-box max-w-2xl">
      <h3 class="text-lg font-bold mb-4">Settings</h3>

      <div class="divider text-sm font-semibold">Global Defaults</div>

      <div class="form-control mb-4">
        <label class="label">
          <span class="label-text">Permission Mode</span>
        </label>
        <select v-model="localPermMode" class="select select-bordered">
          <option v-for="m in permissionModes" :key="m.value" :value="m.value">
            {{ m.label }}
          </option>
        </select>
        <label class="label">
          <span class="label-text-alt text-base-content/50">
            {{ permissionModes.find(m => m.value === localPermMode)?.desc }}
          </span>
        </label>
        <div v-if="localPermMode === 'bypassPermissions'" class="alert alert-warning mt-2 text-sm py-2">
          This allows Claude to run any command without asking. Only use on trusted projects.
        </div>
      </div>

      <div class="form-control mb-4">
        <label class="label">
          <span class="label-text">Notification Sound</span>
        </label>
        <div class="flex gap-2">
          <select v-model="localSound" class="select select-bordered flex-1">
            <option v-for="s in SOUNDS" :key="s" :value="s">{{ s }}</option>
            <option value="">None</option>
          </select>
          <button class="btn btn-outline btn-sm" :disabled="!localSound" @click="previewSound(localSound)">
            Preview
          </button>
        </div>
        <p v-if="soundError" class="text-error text-xs mt-1">Sound error — {{ soundError }}</p>
      </div>

      <div class="form-control mb-4">
        <label class="label">
          <span class="label-text">Theme</span>
        </label>
        <select v-model="localTheme" class="select select-bordered">
          <option v-for="t in themes" :key="t" :value="t">{{ t }}</option>
        </select>
      </div>

      <!-- Providers (native chat: Ollama / OpenRouter) -->
      <div class="divider text-sm font-semibold">Chat Providers</div>

      <div class="form-control mb-4">
        <label class="label">
          <span class="label-text">Ollama base URL</span>
        </label>
        <input
          v-model="localOllamaBaseURL"
          type="text"
          placeholder="http://localhost:11434"
          class="input input-bordered"
        />
      </div>

      <div class="form-control mb-4">
        <label class="label">
          <span class="label-text">OpenRouter API key</span>
          <span v-if="hasOpenRouterKey" class="label-text-alt text-success">Key set ✓</span>
        </label>
        <div class="flex gap-2">
          <input
            v-model="openRouterKeyInput"
            type="password"
            :placeholder="hasOpenRouterKey ? '•••••••• (stored)' : 'sk-or-...'"
            class="input input-bordered flex-1"
            autocomplete="off"
          />
          <button class="btn btn-outline btn-sm" :disabled="!openRouterKeyInput.trim()" @click="saveOpenRouterKey">
            Save key
          </button>
          <button v-if="hasOpenRouterKey" class="btn btn-ghost btn-sm" @click="clearOpenRouterKey">
            Clear
          </button>
        </div>
        <p v-if="providerError" class="text-error text-xs mt-1">{{ providerError }}</p>
        <p class="text-xs text-base-content/50 mt-1">
          The key is stored locally in ~/.tiramisu/secrets.json (0600) and never leaves the backend.
        </p>
      </div>

      <div class="form-control mb-4">
        <label class="label cursor-pointer justify-start gap-3">
          <input type="checkbox" v-model="localDisableThinking" class="checkbox checkbox-sm" />
          <span class="label-text">Disable model "thinking" (Ollama)</span>
        </label>
        <p class="text-xs text-base-content/50 ml-9">
          Sends reasoning_effort:none — verbose reasoning models (e.g. gemma) reply directly
          instead of streaming long thoughts that can overflow the context.
        </p>
      </div>

      <div class="form-control mb-4">
        <label class="label">
          <span class="label-text">Agent instructions (preprompt)</span>
        </label>
        <textarea
          v-model="localCustomInstructions"
          rows="3"
          placeholder="e.g. Always confirm which file you're working with, understand the codebase first, and never make destructive changes."
          class="textarea textarea-bordered w-full text-sm"
        />
        <p class="text-xs text-base-content/50 mt-1">
          Appended to the native agent's built-in system prompt (which already says to read
          files first, understand the codebase, and avoid destructive changes). Add any
          extra rules here.
        </p>
      </div>

      <!-- Session overrides -->
      <template v-if="windowSession">
        <div class="divider text-sm font-semibold">Session Overrides</div>
        <p class="text-sm text-base-content/50 mb-3">
          Override global defaults for this session only.
        </p>

        <div class="form-control mb-3">
          <label class="label cursor-pointer justify-start gap-3">
            <input type="checkbox" v-model="overrideTheme" class="checkbox checkbox-sm" />
            <span class="label-text">Override theme for this session</span>
          </label>
          <select
            v-if="overrideTheme"
            v-model="localSessionTheme"
            class="select select-bordered select-sm mt-1 ml-9"
          >
            <option v-for="t in themes" :key="t" :value="t">{{ t }}</option>
          </select>
        </div>

        <div class="form-control mb-3">
          <label class="label cursor-pointer justify-start gap-3">
            <input type="checkbox" v-model="overrideSound" class="checkbox checkbox-sm" />
            <span class="label-text">Override sound for this session</span>
          </label>
          <div v-if="overrideSound" class="flex gap-2 mt-1 ml-9">
            <select v-model="localSessionSound" class="select select-bordered select-sm flex-1">
              <option v-for="s in SOUNDS" :key="s" :value="s">{{ s }}</option>
              <option value="">None</option>
            </select>
            <button class="btn btn-outline btn-xs" :disabled="!localSessionSound" @click="previewSound(localSessionSound)">
              Preview
            </button>
          </div>
        </div>

        <div class="form-control mb-3">
          <label class="label cursor-pointer justify-start gap-3">
            <input type="checkbox" v-model="overridePermMode" class="checkbox checkbox-sm" />
            <span class="label-text">Override permission mode for this session</span>
          </label>
          <select
            v-if="overridePermMode"
            v-model="localSessionPermMode"
            class="select select-bordered select-sm mt-1 ml-9"
          >
            <option v-for="m in permissionModes" :key="m.value" :value="m.value">
              {{ m.label }}
            </option>
          </select>
        </div>
      </template>

      <div class="divider">Profiles</div>

      <div class="mb-4">
        <p class="text-sm text-base-content/60 mb-3">
          Profiles let you use different Claude accounts by setting separate HOME directories.
          Each profile stores its own auth tokens and session history.
        </p>

        <!-- Existing profiles -->
        <div v-if="localProfiles.length > 0" class="space-y-2 mb-4">
          <div
            v-for="profile in localProfiles"
            :key="profile.id"
            class="flex items-center gap-3 bg-base-200 rounded-lg px-3 py-2"
          >
            <div class="flex-1 min-w-0">
              <div class="font-medium text-sm">{{ profile.name }}</div>
              <div class="text-xs font-mono text-base-content/50 truncate">{{ profile.homeDir }}</div>
            </div>
            <button class="btn btn-ghost btn-xs text-error" @click="removeProfile(profile.id)">
              Remove
            </button>
          </div>
        </div>

        <!-- Add new profile -->
        <div class="bg-base-200 rounded-lg p-3">
          <div class="text-sm font-medium mb-2">Add Profile</div>
          <div class="flex gap-2 mb-2">
            <input
              v-model="newProfileName"
              type="text"
              placeholder="Profile name (e.g. Work)"
              class="input input-bordered input-sm flex-1"
            />
          </div>
          <div class="flex gap-2">
            <input
              v-model="newProfileHomeDir"
              type="text"
              placeholder="Home directory (e.g. /home/user/.claude-work)"
              class="input input-bordered input-sm flex-1 font-mono"
            />
            <button class="btn btn-outline btn-sm" @click="browseHomeDir">Browse</button>
          </div>
          <button
            class="btn btn-sm btn-outline mt-2"
            :disabled="!newProfileName.trim() || !newProfileHomeDir.trim()"
            @click="addProfile"
          >
            Add Profile
          </button>
        </div>
      </div>

      <div class="modal-action">
        <button class="btn btn-ghost" @click="close">Cancel</button>
        <button class="btn btn-primary" @click="save">Save</button>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop" @click="close">
      <button>close</button>
    </form>
  </dialog>
</template>
