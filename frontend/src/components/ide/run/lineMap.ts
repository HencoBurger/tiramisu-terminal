// Shared line-mapping helpers. Providers use these to map a runnable thing to an
// editor line; the editor/terminal plumbing never sees a regex or a JSON key.

export function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

// 1-based line where a JSON key block begins, e.g. findBlockLine(lines, 'scripts').
// Returns -1 if not found.
export function findBlockLine(lines: string[], key: string): number {
  const re = new RegExp(`^\\s*"${escapeRegExp(key)}"\\s*:`)
  for (let i = 0; i < lines.length; i++) if (re.test(lines[i])) return i
  return -1
}

// 1-based line of a quoted key at/after a starting line index (scoped to a block).
export function lineOfKey(lines: string[], fromIndex: number, key: string): number {
  const re = new RegExp(`^\\s*"${escapeRegExp(key)}"\\s*:`)
  for (let i = Math.max(0, fromIndex); i < lines.length; i++) {
    if (re.test(lines[i])) return i + 1
  }
  return Math.max(1, fromIndex + 1)
}

// Generic "match a regex per line" mapper for code (Go funcs, Python defs, etc.).
export function linesMatching(
  content: string,
  re: RegExp,
): { line: number; match: RegExpMatchArray }[] {
  const out: { line: number; match: RegExpMatchArray }[] = []
  content.split('\n').forEach((text, i) => {
    const m = text.match(re)
    if (m) out.push({ line: i + 1, match: m })
  })
  return out
}
