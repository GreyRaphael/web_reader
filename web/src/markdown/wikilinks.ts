import type MarkdownIt from 'markdown-it'
import { resolveWorkspacePath } from '@/utils/path'
import { iconSvg } from '@/utils/icons'

export function wikiLinksPlugin(md: MarkdownIt, currentPath: string): void {
  md.inline.ruler.after('emphasis', 'wiki_links', (state, silent) => {
    const max = state.posMax
    const start = state.pos

    if (
      state.src.charCodeAt(start) !== 0x5b /* [ */ ||
      state.src.charCodeAt(start + 1) !== 0x5b /* [ */
    ) {
      return false
    }

    let end = -1
    for (let i = start + 2; i < max - 1; i++) {
      if (
        state.src.charCodeAt(i) === 0x5d /* ] */ &&
        state.src.charCodeAt(i + 1) === 0x5d /* ] */
      ) {
        end = i
        break
      }
    }

    if (end === -1) return false

    const rawContent = state.src.slice(start + 2, end).trim()
    if (!rawContent) return false

    if (!silent) {
      let targetPart = rawContent
      let label = ''
      const pipeIndex = rawContent.indexOf('|')
      if (pipeIndex !== -1) {
        targetPart = rawContent.slice(0, pipeIndex).trim()
        label = rawContent.slice(pipeIndex + 1).trim()
      }

      let targetDoc = targetPart
      let hash = ''
      const hashIndex = targetPart.indexOf('#')
      if (hashIndex !== -1) {
        targetDoc = targetPart.slice(0, hashIndex).trim()
        hash = targetPart.slice(hashIndex + 1).trim()
      }

      if (!label) {
        label = rawContent
      }

      let targetPath = ''
      if (targetDoc) {
        let resolvedDoc = targetDoc
        if (!resolvedDoc.endsWith('.md') && !resolvedDoc.includes('.')) {
          resolvedDoc = `${resolvedDoc}.md`
        }
        targetPath = resolveWorkspacePath(currentPath, resolvedDoc) || resolvedDoc
      } else {
        targetPath = currentPath
      }

      const tokenOpen = state.push('wiki_link_open', 'a', 1)
      const query = new URLSearchParams({ path: targetPath })
      const href = `?${query.toString()}${hash ? `#${encodeURIComponent(hash)}` : ''}`

      tokenOpen.attrs = [
        ['class', 'wiki-link'],
        ['data-reader-path', targetPath],
        ['href', href],
        ['title', `跳转到知识库文档: ${targetDoc || currentPath}${hash ? ' #' + hash : ''}`],
      ]
      if (hash) {
        tokenOpen.attrs.push(['data-reader-hash', hash])
      }

      const tokenText = state.push('text', '', 0)
      tokenText.content = label

      state.push('wiki_link_close', 'a', -1)
    }

    state.pos = end + 2
    return true
  })

  md.renderer.rules.wiki_link_open = (tokens, index) => {
    const token = tokens[index]
    if (!token) return '<a>'
    let attrs = ''
    for (const [key, val] of token.attrs || []) {
      attrs += ` ${key}="${md.utils.escapeHtml(val)}"`
    }
    return `<a${attrs}><span class="wiki-link-icon" aria-hidden="true">${iconSvg('link', 12)}</span>`
  }

  md.renderer.rules.wiki_link_close = () => '</a>'
}
