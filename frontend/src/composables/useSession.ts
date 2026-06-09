import { SessionStart, SessionSend, SessionStop, GetSessionID, SessionResume } from '../../wailsjs/go/main/App'

export function useSession() {
  async function startSession(tabID: string, workDir: string, prompt: string, profileId = '', model = '') {
    await SessionStart(tabID, workDir, prompt, profileId, model)
  }

  async function sendMessage(tabID: string, message: string, profileId = '', model = '') {
    await SessionSend(tabID, message, profileId, model)
  }

  async function resumeSession(tabID: string, workDir: string, sessionId: string, message: string, profileId = '', model = '') {
    await SessionResume(tabID, workDir, sessionId, message, profileId, model)
  }

  async function stopSession(tabID: string) {
    await SessionStop(tabID)
  }

  async function getSessionID(tabID: string): Promise<string> {
    return await GetSessionID(tabID)
  }

  return { startSession, sendMessage, resumeSession, stopSession, getSessionID }
}
