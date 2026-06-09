const audioCache = new Map<string, HTMLAudioElement>()

const SOUNDS = ['ding', 'chime', 'pop'] as const
export type SoundName = typeof SOUNDS[number]

export function useSound() {
  function getAudio(name: string): HTMLAudioElement {
    if (!audioCache.has(name)) {
      const audio = new Audio(`/sounds/${name}.wav`)
      audioCache.set(name, audio)
    }
    return audioCache.get(name)!
  }

  function play(name: string) {
    const audio = getAudio(name)
    audio.currentTime = 0
    audio.play().catch(() => {
      // Audio play can fail if no user interaction yet
    })
  }

  return { play, SOUNDS }
}
