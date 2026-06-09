export interface SlashCommand {
  name: string
  description: string
  action: string
}

export const slashCommands: SlashCommand[] = [
  { name: '/clear', description: 'Clear chat messages', action: 'clear' },
  { name: '/stop', description: 'Stop the current session', action: 'stop' },
  { name: '/new', description: 'Open a new tab', action: 'new' },
  { name: '/rename', description: 'Rename current tab', action: 'rename' },
  { name: '/workdir', description: 'Change working directory', action: 'workdir' },
  { name: '/sessions', description: 'Browse saved sessions', action: 'sessions' },
  { name: '/settings', description: 'Open settings panel', action: 'settings' },
  { name: '/debug', description: 'Toggle debug drawer', action: 'debug' },
  { name: '/plan', description: 'Toggle plan mode (think before acting)', action: 'plan' },
  { name: '/compact', description: 'Compact conversation into summary', action: 'compact' },
  { name: '/diff', description: 'Show git diff for working directory', action: 'diff' },
  { name: '/model', description: 'Switch model (sonnet/opus/haiku)', action: 'model' },
  { name: '/help', description: 'Show all available commands', action: 'help' },
  { name: '/status', description: 'Show current session status', action: 'status' },
]
