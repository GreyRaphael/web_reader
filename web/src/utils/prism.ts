import Prism from 'prismjs'

// Import autoloader plugin
import 'prismjs/plugins/autoloader/prism-autoloader'

// Pre-load common languages for instant offline/bundled rendering
import 'prismjs/components/prism-clike'
import 'prismjs/components/prism-javascript'
import 'prismjs/components/prism-typescript'
import 'prismjs/components/prism-jsx'
import 'prismjs/components/prism-tsx'
import 'prismjs/components/prism-python'
import 'prismjs/components/prism-toml'
import 'prismjs/components/prism-go'
import 'prismjs/components/prism-json'
import 'prismjs/components/prism-yaml'
import 'prismjs/components/prism-bash'
import 'prismjs/components/prism-sql'
import 'prismjs/components/prism-css'
import 'prismjs/components/prism-markup'
import 'prismjs/components/prism-markdown'
import 'prismjs/components/prism-rust'
import 'prismjs/components/prism-c'
import 'prismjs/components/prism-cpp'
import 'prismjs/components/prism-docker'
import 'prismjs/components/prism-ini'
import 'prismjs/components/prism-java'
import 'prismjs/components/prism-ruby'

// Set autoloader fallback CDN path for rare languages not statically bundled
if (Prism.plugins && Prism.plugins.autoloader) {
  Prism.plugins.autoloader.languages_path =
    'https://cdnjs.cloudflare.com/ajax/libs/prism/1.29.0/components/'
}

export const EXT_TO_LANGUAGE_MAP: Record<string, string> = {
  ts: 'typescript',
  tsx: 'tsx',
  js: 'javascript',
  jsx: 'jsx',
  py: 'python',
  toml: 'toml',
  go: 'go',
  json: 'json',
  jsonl: 'json',
  yaml: 'yaml',
  yml: 'yaml',
  rs: 'rust',
  sh: 'bash',
  bash: 'bash',
  zsh: 'bash',
  sql: 'sql',
  css: 'css',
  html: 'markup',
  xml: 'markup',
  vue: 'markup',
  md: 'markdown',
  markdown: 'markdown',
  c: 'c',
  h: 'c',
  cpp: 'cpp',
  hpp: 'cpp',
  java: 'java',
  rb: 'ruby',
  ini: 'ini',
  conf: 'ini',
  dockerfile: 'docker',
}

export function getLanguageFromPath(filepath: string): string {
  const filename = filepath.split('/').pop() || ''
  if (filename.toLowerCase() === 'dockerfile') return 'docker'

  const ext = filename.includes('.') ? filename.split('.').pop()?.toLowerCase() || '' : ''
  return EXT_TO_LANGUAGE_MAP[ext] || ext || 'plaintext'
}

export function highlightCode(code: string, lang: string): string {
  const normalizedLang = lang.toLowerCase().trim()
  const prismLang = EXT_TO_LANGUAGE_MAP[normalizedLang] || normalizedLang

  if (prismLang && Prism.languages[prismLang]) {
    try {
      return Prism.highlight(code, Prism.languages[prismLang], prismLang)
    } catch {
      // Fallback on error
    }
  }

  // Fallback to HTML escaped text
  return escapeHtml(code)
}

export function escapeHtml(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;')
}

export default Prism
