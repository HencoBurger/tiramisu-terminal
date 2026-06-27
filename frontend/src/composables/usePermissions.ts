import { reactive } from 'vue'

export interface PermRequest {
  reqId: string
  toolName: string
  toolInput: string
}

// Per-tab pending permission request. The Go agent loop blocks on one at a time per
// tab, so a single request per tab is sufficient.
const pending = reactive<Record<string, PermRequest>>({})

export function usePermissions() {
  function set(tabId: string, req: PermRequest) {
    pending[tabId] = req
  }
  function clear(tabId: string) {
    delete pending[tabId]
  }
  return { pending, set, clear }
}
