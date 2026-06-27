import { ref, computed } from 'vue'
import {
  GetGlobalConfig,
  SaveGlobalConfig,
  SaveWindowSession,
  SetWindowTitle,
} from '../../wailsjs/go/main/App'
import { main } from '../../wailsjs/go/models'
import type { GlobalConfig, WindowSession, TabConfig, EffectiveConfig } from '../types/session'

const globalConfig = ref<GlobalConfig>({
  defaultSound: 'ding',
  theme: 'dark',
  permissionMode: 'default',
  profiles: [],
  ollamaBaseURL: 'http://localhost:11434',
  enabledProviders: ['claude', 'ollama', 'openrouter'],
  defaultModels: {},
  disableThinking: false,
})

const windowSession = ref<WindowSession | null>(null)

const effectiveConfig = computed<EffectiveConfig>(() => {
  const gc = globalConfig.value
  const ws = windowSession.value
  return {
    theme: (ws?.themeOverride) || gc.theme || 'dark',
    defaultSound: (ws?.soundOverride) || gc.defaultSound || 'ding',
    permissionMode: (ws?.permModeOverride) || gc.permissionMode || 'default',
    profiles: gc.profiles || [],
  }
})

export function useConfig() {
  async function loadGlobalConfig() {
    try {
      const c = await GetGlobalConfig() as any
      globalConfig.value = {
        defaultSound: c.defaultSound || 'ding',
        theme: c.theme || 'dark',
        permissionMode: c.permissionMode || 'default',
        profiles: c.profiles || [],
        ollamaBaseURL: c.ollamaBaseURL || 'http://localhost:11434',
        enabledProviders: c.enabledProviders || ['claude', 'ollama', 'openrouter'],
        defaultModels: c.defaultModels || {},
        disableThinking: !!c.disableThinking,
      }
      document.documentElement.setAttribute('data-theme', effectiveConfig.value.theme)
    } catch (e) {
      console.error('Failed to load global config:', e)
    }
  }

  async function saveGlobalConfig(c: GlobalConfig) {
    globalConfig.value = c
    document.documentElement.setAttribute('data-theme', effectiveConfig.value.theme)
    await SaveGlobalConfig(new main.GlobalConfig(c))
  }

  function setWindowSession(session: WindowSession) {
    windowSession.value = session
    document.documentElement.setAttribute('data-theme', effectiveConfig.value.theme)
  }

  async function saveWindowSession() {
    if (!windowSession.value) return
    try {
      await SaveWindowSession(new main.WindowSession(windowSession.value))
    } catch (e) {
      console.error('Failed to save window session:', e)
    }
  }

  async function maybeSetDefaultWorkDir(dir: string) {
    if (!windowSession.value || windowSession.value.defaultWorkDir || !dir) return
    windowSession.value.defaultWorkDir = dir
    await saveWindowSession()
  }

  async function saveTabState(tabs: TabConfig[]) {
    if (!windowSession.value) return
    windowSession.value.tabs = tabs
    await saveWindowSession()
  }

  async function saveWindowName(name: string) {
    if (!windowSession.value) return
    windowSession.value.name = name || 'Untitled'
    await saveWindowSession()
    SetWindowTitle(windowSession.value.name).catch(() => {})
  }

  return {
    globalConfig,
    windowSession,
    effectiveConfig,
    loadGlobalConfig,
    saveGlobalConfig,
    setWindowSession,
    saveWindowSession,
    maybeSetDefaultWorkDir,
    saveTabState,
    saveWindowName,
  }
}
