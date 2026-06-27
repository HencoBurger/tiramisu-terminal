import type { main } from '../../../../wailsjs/go/models'

// One runnable thing surfaced as a gutter "play" glyph.
export interface RunAction {
  id: string // stable, e.g. "js:script:dev" — used for click resolution + dedup
  kind: string // 'script' | 'test' | 'main' | 'build' ... informational/grouping
  label: string // short; becomes the terminal tab name: "▶ " + label
  line: number // 1-based editor line for the glyph
  command: string // exact shell command, e.g. "pnpm run dev"
  cwd: string // absolute working dir the command runs in
  tooltip?: string // glyph hover (markdown allowed)
}

// Everything a provider needs. The FS probe is injected so providers stay pure
// and the editor/terminal plumbing owns no language knowledge.
export interface ProviderContext {
  path: string // absolute file path
  fileName: string // basename, e.g. "package.json"
  dir: string // dirname of the file (monorepo-safe cwd)
  content: string // current editor text
  language: string // monaco language id (from guessLanguage)
  workDir: string // the IDE tab's root working dir
  listDir: (path: string) => Promise<main.FileEntry[]> // = ListDirectory
}

export interface RunProvider {
  id: string
  // Cheap synchronous gate. A provider self-declares what it handles — by
  // filename, language id, or extension. This is the extensibility seam.
  matches(ctx: Pick<ProviderContext, 'fileName' | 'language' | 'path'>): boolean
  // Async: may probe the filesystem (lockfiles, manifests).
  getActions(ctx: ProviderContext): Promise<RunAction[]>
}
