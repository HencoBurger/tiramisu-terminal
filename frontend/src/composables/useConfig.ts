import { ref } from 'vue'
import { GetConfig, SaveConfig, SaveTabConfigs, SetProjectName } from '../../wailsjs/go/main/App'
import { main } from '../../wailsjs/go/models'
import type { AppConfig, TabConfig } from '../types/session'

const config = ref<AppConfig>({
  defaultSound: 'ding',
  theme: 'dark',
  permissionMode: 'default',
  projectName: 'Tiramisu',
  tabs: [],
  profiles: [],
})

const projectName = ref('Tiramisu')

export function useConfig() {
  async function loadConfig() {
    try {
      const c = await GetConfig() as any
      config.value = {
        defaultSound: c.defaultSound || 'ding',
        theme: c.theme || 'dark',
        permissionMode: c.permissionMode || 'default',
        projectName: c.projectName || 'Tiramisu',
        tabs: c.tabs || [],
        profiles: c.profiles || [],
      }
      projectName.value = c.projectName || 'Tiramisu'
      // Apply theme
      document.documentElement.setAttribute('data-theme', c.theme || 'dark')
    } catch (e) {
      console.error('Failed to load config:', e)
    }
  }

  async function saveFullConfig(c: AppConfig) {
    config.value = c
    document.documentElement.setAttribute('data-theme', c.theme || 'dark')
    await SaveConfig(new main.AppConfig(c))
  }

  async function saveTabState(tabs: TabConfig[]) {
    try {
      await SaveTabConfigs(tabs.map(t => new main.TabConfig(t)))
    } catch (e) {
      console.error('Failed to save tab configs:', e)
    }
  }

  async function saveProjectName(name: string) {
    projectName.value = name || 'Tiramisu'
    config.value.projectName = name
    try {
      await SetProjectName(name)
    } catch (e) {
      console.error('Failed to save project name:', e)
    }
  }

  return { config, projectName, loadConfig, saveFullConfig, saveTabState, saveProjectName }
}
