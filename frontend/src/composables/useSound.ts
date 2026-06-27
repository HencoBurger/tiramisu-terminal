import { PlaySound } from '../../wailsjs/go/main/App'

const SOUNDS = ['ding', 'chime', 'pop'] as const
export type SoundName = typeof SOUNDS[number]

export function useSound() {
  // Plays via the Go backend (system audio player). WebKitGTK won't play media
  // served over the Wails asset scheme, so HTML5 <audio> is unreliable here.
  // Returns null on success, or the Error if playback failed (never rejects).
  async function play(name: string): Promise<Error | null> {
    try {
      await PlaySound(name)
      return null
    } catch (e) {
      console.warn(`sound "${name}" failed to play:`, e)
      return e instanceof Error ? e : new Error(String(e))
    }
  }

  return { play, SOUNDS }
}
