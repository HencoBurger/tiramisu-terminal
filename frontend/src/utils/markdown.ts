import { marked, type Renderer } from 'marked'
import hljs from 'highlight.js'

// Global copy function for inline onclick handlers (v-html doesn't bind Vue events)
;(window as any).__copyCode = function (btn: HTMLButtonElement) {
  const wrapper = btn.closest('.code-block-wrapper')
  const code = wrapper?.querySelector('code')
  if (!code) return
  navigator.clipboard.writeText(code.textContent || '').then(() => {
    btn.textContent = 'Copied!'
    setTimeout(() => (btn.textContent = 'Copy'), 1500)
  })
}

const renderer: Partial<Renderer> = {
  code({ text, lang }: { text: string; lang?: string }) {
    let highlighted: string
    if (lang && hljs.getLanguage(lang)) {
      highlighted = hljs.highlight(text, { language: lang }).value
    } else {
      highlighted = hljs.highlightAuto(text).value
    }
    return `<div class="code-block-wrapper"><button class="copy-btn" onclick="window.__copyCode(this)">Copy</button><pre><code class="hljs language-${lang || ''}">${highlighted}</code></pre></div>`
  },
}

marked.use({ renderer })

export function renderMarkdown(text: string): string {
  if (!text) return ''
  return marked.parse(text, { async: false }) as string
}
