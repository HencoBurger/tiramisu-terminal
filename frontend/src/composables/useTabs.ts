import { ref, computed } from 'vue'
import type { TabState, ChatMessage, SessionStatus, TabConfig, TabType } from '../types/session'

const tabs = ref<TabState[]>([])
const activeTabId = ref<string>('')

let tabCounter = 0

function generateId(): string {
  return `tab-${Date.now()}-${++tabCounter}`
}

function generateMessageId(): string {
  return `msg-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`
}

export function useTabs() {
  const activeTab = computed(() =>
    tabs.value.find(t => t.id === activeTabId.value) ?? null
  )

  function addTab(workDir = '', name = '', type: TabType = 'chat'): TabState {
    const id = generateId()
    const tab: TabState = {
      id,
      name: name || `Tab ${tabs.value.length + 1}`,
      workDir,
      sessionId: '',
      status: 'idle',
      messages: [],
      totalCost: 0,
      soundOverride: '',
      profileId: '',
      planMode: false,
      model: '',
      type,
      activity: false,
    }
    tabs.value.push(tab)
    activeTabId.value = id
    return tab
  }

  function removeTab(id: string) {
    const idx = tabs.value.findIndex(t => t.id === id)
    if (idx === -1) return

    tabs.value.splice(idx, 1)

    if (activeTabId.value === id) {
      if (tabs.value.length > 0) {
        const newIdx = Math.min(idx, tabs.value.length - 1)
        activeTabId.value = tabs.value[newIdx].id
      } else {
        activeTabId.value = ''
      }
    }
  }

  function setActiveTab(id: string) {
    activeTabId.value = id
  }

  function renameTab(id: string, name: string) {
    const tab = tabs.value.find(t => t.id === id)
    if (tab) tab.name = name
  }

  function getTab(id: string): TabState | undefined {
    return tabs.value.find(t => t.id === id)
  }

  function setTabStatus(id: string, status: SessionStatus) {
    const tab = tabs.value.find(t => t.id === id)
    if (tab) tab.status = status
  }

  function setTabSessionId(id: string, sessionId: string) {
    const tab = tabs.value.find(t => t.id === id)
    if (tab) tab.sessionId = sessionId
  }

  function setTabWorkDir(id: string, workDir: string) {
    const tab = tabs.value.find(t => t.id === id)
    if (tab) tab.workDir = workDir
  }

  function setTabProfile(id: string, profileId: string) {
    const tab = tabs.value.find(t => t.id === id)
    if (tab) tab.profileId = profileId
  }

  function setTabPlanMode(id: string, enabled: boolean) {
    const tab = tabs.value.find(t => t.id === id)
    if (tab) tab.planMode = enabled
  }

  function setTabModel(id: string, model: string) {
    const tab = tabs.value.find(t => t.id === id)
    if (tab) tab.model = model
  }

  function setTabActivity(id: string, active: boolean) {
    const tab = tabs.value.find(t => t.id === id)
    if (tab) tab.activity = active
  }

  function addMessage(tabId: string, msg: Omit<ChatMessage, 'id'>): ChatMessage {
    const tab = tabs.value.find(t => t.id === tabId)
    if (!tab) throw new Error(`Tab ${tabId} not found`)
    const message: ChatMessage = { ...msg, id: generateMessageId() }
    tab.messages.push(message)
    return message
  }

  function updateLastAssistantMessage(tabId: string, updater: (msg: ChatMessage) => void) {
    const tab = tabs.value.find(t => t.id === tabId)
    if (!tab) return
    for (let i = tab.messages.length - 1; i >= 0; i--) {
      if (tab.messages[i].role === 'assistant') {
        updater(tab.messages[i])
        return
      }
    }
  }

  function autoNameTab(id: string, prompt: string) {
    const tab = tabs.value.find(t => t.id === id)
    if (!tab) return
    // Only auto-name if still has default name pattern "Tab N"
    if (!/^Tab \d+$/.test(tab.name)) return
    const truncated = prompt.trim().slice(0, 30)
    tab.name = truncated.length < prompt.trim().length ? truncated + '…' : truncated
  }

  function setTabCost(id: string, cost: number) {
    const tab = tabs.value.find(t => t.id === id)
    if (tab) tab.totalCost = cost
  }

  function getTabConfigs(): TabConfig[] {
    return tabs.value.map(t => ({
      id: t.id,
      name: t.name,
      workDir: t.workDir,
      sessionId: t.sessionId,
      soundOverride: t.soundOverride,
      profileId: t.profileId,
      model: t.model,
      type: t.type,
    }))
  }

  function restoreTabs(configs: TabConfig[]) {
    const seen = new Set<string>()
    for (const cfg of configs) {
      if (seen.has(cfg.id)) continue
      seen.add(cfg.id)
      const tab: TabState = {
        id: cfg.id,
        name: cfg.name,
        workDir: cfg.workDir,
        sessionId: cfg.sessionId,
        status: 'idle',
        messages: [],
        totalCost: 0,
        soundOverride: cfg.soundOverride,
        profileId: cfg.profileId || '',
        planMode: false,
        model: cfg.model || '',
        type: cfg.type || 'chat',
        activity: false,
      }
      tabs.value.push(tab)
    }
    if (tabs.value.length > 0 && !activeTabId.value) {
      activeTabId.value = tabs.value[0].id
    }
  }

  return {
    tabs,
    activeTabId,
    activeTab,
    addTab,
    removeTab,
    setActiveTab,
    renameTab,
    autoNameTab,
    getTab,
    setTabStatus,
    setTabSessionId,
    setTabWorkDir,
    setTabProfile,
    setTabPlanMode,
    setTabModel,
    setTabActivity,
    addMessage,
    updateLastAssistantMessage,
    setTabCost,
    getTabConfigs,
    restoreTabs,
  }
}
