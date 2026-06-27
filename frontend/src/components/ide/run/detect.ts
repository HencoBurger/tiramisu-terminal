import type { ProviderContext } from './types'

// Language-agnostic toolchain/manifest detection. Providers declare what marker
// files identify their runner; this layer just probes the filesystem.
//
// NOTE: the bound ListDirectory skips dotfiles, so dotfile markers (e.g. Python
// `.venv`, `.tool-versions`) are invisible here. When that's needed, add a tiny
// `PathExists(path) bool` Go binding — only this file would change.

// Cache directory listings within a single compute pass so multiple providers
// probing the same dir don't trigger repeated Go calls.
const cache = new Map<string, Promise<Set<string>>>()

export function resetDetectCache(): void {
  cache.clear()
}

export async function dirFileNames(ctx: ProviderContext, dir: string): Promise<Set<string>> {
  let p = cache.get(dir)
  if (!p) {
    p = ctx
      .listDir(dir)
      .then((entries) => new Set(entries.filter((e) => !e.isDir).map((e) => e.name)))
      .catch(() => new Set<string>())
    cache.set(dir, p)
  }
  return p
}

export interface MarkerRule<T> {
  file: string
  value: T
}

// "First marker file present wins" resolver — keeps detection declarative and
// language-agnostic.
export async function resolveByMarker<T>(
  ctx: ProviderContext,
  dir: string,
  rules: MarkerRule<T>[],
  fallback: T,
): Promise<T> {
  const names = await dirFileNames(ctx, dir)
  for (const r of rules) if (names.has(r.file)) return r.value
  return fallback
}
