import type { RunProvider, RunAction, ProviderContext } from '../types'
import { findBlockLine, lineOfKey } from '../lineMap'
import { resolveByMarker, type MarkerRule } from '../detect'

interface PM {
  run: string // command prefix, e.g. "pnpm run"
  label: string // package manager name for the tooltip
}

// Lockfiles are NOT dotfiles, so ListDirectory surfaces them. First match wins.
const PM_RULES: MarkerRule<PM>[] = [
  { file: 'bun.lockb', value: { run: 'bun run', label: 'bun' } },
  { file: 'pnpm-lock.yaml', value: { run: 'pnpm run', label: 'pnpm' } },
  { file: 'yarn.lock', value: { run: 'yarn', label: 'yarn' } },
  { file: 'package-lock.json', value: { run: 'npm run', label: 'npm' } },
]
const PM_FALLBACK: PM = { run: 'npm run', label: 'npm' }

export const jsProvider: RunProvider = {
  id: 'js',
  matches: ({ fileName }) => fileName === 'package.json',
  async getActions(ctx: ProviderContext): Promise<RunAction[]> {
    let pkg: any
    try {
      pkg = JSON.parse(ctx.content)
    } catch {
      return [] // invalid JSON mid-edit — no glyphs
    }
    const scripts = pkg?.scripts
    if (!scripts || typeof scripts !== 'object') return []

    const pm = await resolveByMarker(ctx, ctx.dir, PM_RULES, PM_FALLBACK)
    const lines = ctx.content.split('\n')
    const blockStart = findBlockLine(lines, 'scripts')
    if (blockStart < 0) return []

    return Object.keys(scripts).map((name) => ({
      id: `js:script:${name}`,
      kind: 'script',
      label: name,
      line: lineOfKey(lines, blockStart, name),
      command: `${pm.run} ${name}`,
      cwd: ctx.dir, // the package.json's own dir (monorepo-safe)
      tooltip: `Run \`${pm.run} ${name}\` (${pm.label})`,
    }))
  },
}
