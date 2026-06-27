// Map a file path to a Monaco language id from its extension.
export function guessLanguage(path: string): string {
  const ext = path.split('.').pop()?.toLowerCase() ?? ''
  const map: Record<string, string> = {
    ts: 'typescript', tsx: 'typescript', js: 'javascript', jsx: 'javascript',
    vue: 'html', html: 'html', css: 'css', scss: 'scss',
    json: 'json', md: 'markdown', go: 'go', py: 'python',
    rs: 'rust', yaml: 'yaml', yml: 'yaml', toml: 'toml',
    sh: 'shell', bash: 'shell', sql: 'sql', xml: 'xml',
    java: 'java', kt: 'kotlin', rb: 'ruby', php: 'php',
    c: 'c', cpp: 'cpp', h: 'c', hpp: 'cpp',
    mod: 'go', sum: 'plaintext', txt: 'plaintext',
  }
  return map[ext] || 'plaintext'
}
