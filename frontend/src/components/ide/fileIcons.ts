// Resolve vendored material-icon-theme SVGs (src/assets/file-icons/*.svg) to bundled
// URLs. Vite bundles only these files at build time — fully offline, no manifest, no
// prebuild script, no npm dependency. The SVGs are colored, so the tree is scannable
// by file type at a glance.
const modules = import.meta.glob('../../assets/file-icons/*.svg', {
  eager: true,
  query: '?url',
  import: 'default',
}) as Record<string, string>

const icons: Record<string, string> = {}
for (const [path, url] of Object.entries(modules)) {
  const name = path.split('/').pop()!.replace('.svg', '')
  icons[name] = url
}

function iconUrl(name: string): string {
  return icons[name] ?? icons['file']
}

// Exact filename matches (lowercased).
const byName: Record<string, string> = {
  'go.mod': 'go-mod',
  'go.sum': 'go-mod',
  'package.json': 'nodejs',
  'package-lock.json': 'nodejs',
  'tsconfig.json': 'tsconfig',
  'tsconfig.node.json': 'tsconfig',
  'vite.config.ts': 'vite',
  'vite.config.js': 'vite',
  'dockerfile': 'docker',
  '.gitignore': 'git',
  '.gitattributes': 'git',
  'license': 'certificate',
  'license.md': 'certificate',
  'wails.json': 'json',
}

// Extension matches (single, plus a few compound like d.ts).
const byExt: Record<string, string> = {
  go: 'go',
  ts: 'typescript',
  'd.ts': 'typescript-def',
  tsx: 'react_ts',
  js: 'javascript', mjs: 'javascript', cjs: 'javascript', jsx: 'javascript',
  vue: 'vue',
  json: 'json',
  md: 'markdown', markdown: 'markdown',
  yml: 'yaml', yaml: 'yaml',
  toml: 'toml',
  html: 'html', htm: 'html',
  css: 'css',
  scss: 'sass', sass: 'sass',
  py: 'python',
  rs: 'rust',
  sh: 'console', bash: 'console', zsh: 'console',
  lock: 'lock',
  exe: 'exe', dll: 'exe', so: 'exe',
  sql: 'database', db: 'database',
  png: 'image', jpg: 'image', jpeg: 'image', gif: 'image', svg: 'image', webp: 'image', ico: 'image',
  xml: 'xml',
  txt: 'document', log: 'document',
  ini: 'settings', conf: 'settings', cfg: 'settings', env: 'settings',
}

export function getFileIconUrl(fileName: string): string {
  const lower = fileName.toLowerCase()

  if (byName[lower]) return iconUrl(byName[lower])

  const parts = lower.split('.')
  // Compound extension first (e.g. "foo.d.ts").
  if (parts.length > 2) {
    const compound = parts.slice(-2).join('.')
    if (byExt[compound]) return iconUrl(byExt[compound])
  }
  const ext = parts.length > 1 ? parts[parts.length - 1] : ''
  if (ext && byExt[ext]) return iconUrl(byExt[ext])

  return iconUrl('file')
}

// Folders with a recognizable named icon; everything else uses the default folder.
const folderByName: Record<string, string> = {
  src: 'src', source: 'src',
  dist: 'dist', build: 'dist', out: 'dist',
  node_modules: 'node',
  '.git': 'git',
}

export function getFolderIconUrl(folderName: string, expanded: boolean): string {
  const base = folderByName[folderName.toLowerCase()]
  if (base) return iconUrl(expanded ? `folder-${base}-open` : `folder-${base}`)
  return iconUrl(expanded ? 'folder-open' : 'folder')
}
