import {
  AgentStart,
  AgentSend,
  AgentStop,
  AgentPermissionDecision,
  ListProviderModels,
  SetProviderKey,
  HasProviderKey,
  DeleteProviderKey,
} from '../../wailsjs/go/main/App'

// Thin wrapper over the native chat runtime bindings (mirrors useSession.ts for the
// Claude path). Responses arrive asynchronously via the 'agent:event' Wails event.
export function useAgent() {
  async function agentStart(tabId: string, provider: string, model: string, workDir: string, prompt: string) {
    await AgentStart(tabId, provider, model, workDir, prompt)
  }
  async function agentSend(tabId: string, provider: string, model: string, workDir: string, prompt: string) {
    await AgentSend(tabId, provider, model, workDir, prompt)
  }
  async function agentStop(tabId: string) {
    await AgentStop(tabId)
  }
  async function agentPermissionDecision(reqId: string, approved: boolean) {
    await AgentPermissionDecision(reqId, approved)
  }

  return {
    agentStart,
    agentSend,
    agentStop,
    agentPermissionDecision,
    listProviderModels: ListProviderModels,
    setProviderKey: SetProviderKey,
    hasProviderKey: HasProviderKey,
    deleteProviderKey: DeleteProviderKey,
  }
}
