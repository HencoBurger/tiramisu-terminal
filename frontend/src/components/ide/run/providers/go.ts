import type { RunProvider, RunAction, ProviderContext } from '../types'
import { linesMatching } from '../lineMap'
import { resolveByMarker } from '../detect'

// Proof that the abstraction is language-agnostic. NOT registered in v1 — enable
// it with the one-line uncomment in ../index.ts. Note it required ZERO changes to
// the editor/terminal plumbing: it keys by language (not filename), reuses the
// shared line mapper and detection layer, and emits the same RunAction shape.
export const goProvider: RunProvider = {
  id: 'go',
  matches: ({ language }) => language === 'go',
  async getActions(ctx: ProviderContext): Promise<RunAction[]> {
    const actions: RunAction[] = []
    // Run from the module root if go.mod is here, else from the file's dir.
    const cwd = await resolveByMarker(ctx, ctx.dir, [{ file: 'go.mod', value: ctx.dir }], ctx.workDir)

    for (const { line } of linesMatching(ctx.content, /^func\s+main\s*\(\s*\)/)) {
      actions.push({
        id: 'go:main',
        kind: 'main',
        label: 'go run',
        line,
        command: 'go run .',
        cwd,
        tooltip: 'Run `go run .`',
      })
    }
    for (const { line, match } of linesMatching(ctx.content, /^func\s+(Test\w+)\s*\(/)) {
      const name = match[1]
      actions.push({
        id: `go:test:${name}`,
        kind: 'test',
        label: name,
        line,
        command: `go test -run '^${name}$' ./...`,
        cwd,
        tooltip: `Run \`${name}\``,
      })
    }
    return actions
  },
}
