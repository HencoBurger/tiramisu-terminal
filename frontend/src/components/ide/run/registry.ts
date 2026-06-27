import type { RunAction, RunProvider, ProviderContext } from './types'

const providers: RunProvider[] = []

export function registerRunProvider(p: RunProvider): void {
  providers.push(p)
}

// Fan out to every matching provider, isolate failures, dedup by stable id.
export async function computeRunActions(ctx: ProviderContext): Promise<RunAction[]> {
  const matching = providers.filter((p) =>
    p.matches({ fileName: ctx.fileName, language: ctx.language, path: ctx.path }),
  )
  if (!matching.length) return []

  const results = await Promise.all(
    matching.map((p) =>
      p.getActions(ctx).catch((e) => {
        console.error(`run provider ${p.id} failed:`, e)
        return [] as RunAction[]
      }),
    ),
  )

  const seen = new Set<string>()
  return results.flat().filter((a) => !seen.has(a.id) && seen.add(a.id))
}
