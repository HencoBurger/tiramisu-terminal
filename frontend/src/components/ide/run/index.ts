// Run-provider registration hub. Importing this module registers all providers.
// Adding a language is a one-line change here plus its provider file.
import { registerRunProvider } from './registry'
import { jsProvider } from './providers/js'

registerRunProvider(jsProvider)

// Follow-ups — uncomment to enable (the provider files already exist):
// import { goProvider } from './providers/go'
// registerRunProvider(goProvider)

export { computeRunActions } from './registry'
export type { RunAction } from './types'
