<script setup lang="ts">
import { ref, watch } from 'vue'
import { useConfig } from '../composables/useConfig'
import { useSound } from '../composables/useSound'
import type { Profile } from '../types/session'
import { OpenDirectoryDialog } from '../../wailsjs/go/main/App'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
}>()

const { config, saveFullConfig } = useConfig()
const { SOUNDS, play } = useSound()

const localSound = ref(config.value.defaultSound)
const localTheme = ref(config.value.theme)
const localPermMode = ref(config.value.permissionMode || 'default')
const localProfiles = ref<Profile[]>([])

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
    localSound.value = config.value.defaultSound
    localTheme.value = config.value.theme
    localPermMode.value = config.value.permissionMode || 'default'
    localProfiles.value = JSON.parse(JSON.stringify(config.value.profiles || []))
  }
})

function previewSound(name: string) {
  play(name)
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
  await saveFullConfig({
    ...config.value,
    defaultSound: localSound.value,
    theme: localTheme.value,
    permissionMode: localPermMode.value,
    profiles: localProfiles.value,
  })
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
      </div>

      <div class="form-control mb-4">
        <label class="label">
          <span class="label-text">Theme</span>
        </label>
        <select v-model="localTheme" class="select select-bordered">
          <option v-for="t in themes" :key="t" :value="t">{{ t }}</option>
        </select>
      </div>

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
