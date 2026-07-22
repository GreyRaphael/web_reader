import DOMPurify from 'dompurify'
import MarkdownIt from 'markdown-it'
import type Token from 'markdown-it/lib/token.mjs'
import { rawFileUrl } from '@/api/client'
import { createUniqueSlugger } from '@/utils/slug'
import { isRelativeReference, resolveReaderTarget, resolveWorkspacePath } from '@/utils/path'
import { highlightCode } from '@/utils/prism'
import { mathPlugin } from './math'

export interface MarkdownHeading {
  id: string
  title: string
  level: number
}

export interface RenderedMarkdown {
  html: string
  headings: MarkdownHeading[]
}

function taskListPlugin(md: MarkdownIt): void {
  md.core.ruler.after('inline', 'reader_task_lists', (state) => {
    for (let index = 2; index < state.tokens.length; index += 1) {
      const inline = state.tokens[index]
      const paragraph = state.tokens[index - 1]
      const listItem = state.tokens[index - 2]
      if (
        !inline ||
        !paragraph ||
        !listItem ||
        inline.type !== 'inline' ||
        paragraph.type !== 'paragraph_open' ||
        listItem.type !== 'list_item_open'
      ) {
        continue
      }

      const match = /^\[([ xX])\]\s+/u.exec(inline.content)
      const firstChild = inline.children?.[0]
      if (!match || !firstChild || firstChild.type !== 'text') continue

      const checked = match[1]?.toLowerCase() === 'x'
      inline.content = inline.content.slice(match[0].length)
      firstChild.content = firstChild.content.slice(match[0].length)
      const checkbox = new state.Token('html_inline', '', 0)
      checkbox.content = `<span class="task-checkbox" role="checkbox" aria-label="${checked ? '已完成' : '未完成'}" aria-checked="${checked}">${checked ? '✓' : ''}</span>`
      inline.children?.unshift(checkbox)
      listItem.attrJoin('class', 'task-list-item')
    }
  })
}

function inlineText(token: Token): string {
  if (!token.children) return token.content
  return token.children
    .map((child) => {
      if (child.type === 'softbreak' || child.type === 'hardbreak') return ' '
      if (
        ['text', 'code_inline', 'math_inline', 'math_inline_display', 'image'].includes(child.type)
      ) {
        return child.content
      }
      return ''
    })
    .join('')
    .trim()
}

function isExternalReference(reference: string): boolean {
  return /^(?:https?:|mailto:|tel:|\/\/)/i.test(reference)
}

function createMarkdown(currentPath: string, headings: MarkdownHeading[]): MarkdownIt {
  const md: MarkdownIt = new MarkdownIt({
    html: false,
    linkify: true,
    typographer: true,
    breaks: false,
    highlight(code: string, language: string): string {
      return highlightCode(code, language)
    },
  })

  mathPlugin(md)
  taskListPlugin(md)

  const defaultFence = md.renderer.rules.fence?.bind(md.renderer.rules)
  md.renderer.rules.fence = (tokens, index, options, env, renderer) => {
    const token = tokens[index]
    if (!token) return ''
    const language = token.info.trim().split(/\s+/u)[0]?.toLowerCase() ?? ''
    if (language === 'mermaid') {
      const source = md.utils.escapeHtml(token.content)
      return `<div class="mermaid-diagram scroll-surface" tabindex="0"><pre class="mermaid-source" aria-hidden="true">${source}</pre><div class="mermaid-output" role="img" aria-label="Mermaid 图表"></div></div>\n`
    }
    const rendered = defaultFence
      ? defaultFence(tokens, index, options, env, renderer)
      : renderer.renderToken(tokens, index, options)
    const label = language
      ? `<span class="code-language">${md.utils.escapeHtml(language)}</span>`
      : ''
    return `<div class="code-block scroll-surface" tabindex="0">${label}${rendered}</div>`
  }

  md.renderer.rules.table_open = () =>
    '<div class="table-scroll scroll-surface" tabindex="0"><table>\n'
  md.renderer.rules.table_close = () => '</table></div>\n'

  const defaultImage = md.renderer.rules.image?.bind(md.renderer.rules)
  md.renderer.rules.image = (tokens, index, options, env, renderer) => {
    const token = tokens[index]
    if (!token) return ''
    const source = token.attrGet('src') ?? ''
    const path = resolveWorkspacePath(currentPath, source)
    if (path) {
      token.attrSet('src', rawFileUrl(path))
    } else if (isRelativeReference(source)) {
      const sourceIndex = token.attrIndex('src')
      if (sourceIndex >= 0) token.attrs?.splice(sourceIndex, 1)
      token.attrSet('data-invalid-source', 'true')
    }
    token.attrSet('loading', 'lazy')
    token.attrSet('decoding', 'async')
    return defaultImage
      ? defaultImage(tokens, index, options, env, renderer)
      : renderer.renderToken(tokens, index, options)
  }

  const defaultLinkOpen = md.renderer.rules.link_open?.bind(md.renderer.rules)
  md.renderer.rules.link_open = (tokens, index, options, env, renderer) => {
    const token = tokens[index]
    if (!token) return ''
    const href = token.attrGet('href') ?? ''
    const target = resolveReaderTarget(currentPath, href)
    if (target) {
      token.attrSet('data-reader-path', target.path)
      if (target.hash) token.attrSet('data-reader-hash', target.hash)
      const query = new URLSearchParams({ path: target.path })
      token.attrSet(
        'href',
        `?${query.toString()}${target.hash ? `#${encodeURIComponent(target.hash)}` : ''}`,
      )
    } else if (isExternalReference(href)) {
      token.attrSet('target', '_blank')
      token.attrSet('rel', 'noopener noreferrer')
    } else if (isRelativeReference(href)) {
      token.attrSet('rel', 'nofollow')
    }
    return defaultLinkOpen
      ? defaultLinkOpen(tokens, index, options, env, renderer)
      : renderer.renderToken(tokens, index, options)
  }

  md.core.ruler.push('reader_heading_ids', (state) => {
    const uniqueSlug = createUniqueSlugger()
    for (let index = 0; index < state.tokens.length; index += 1) {
      const opening = state.tokens[index]
      if (!opening || opening.type !== 'heading_open') continue
      const inline = state.tokens[index + 1]
      if (!inline || inline.type !== 'inline') continue
      const title = inlineText(inline)
      const level = Number(opening.tag.slice(1))
      const id = uniqueSlug(title)
      opening.attrSet('id', id)
      headings.push({ id, title: title || '未命名章节', level })
    }
  })

  return md
}

export function renderMarkdown(source: string, currentPath: string): RenderedMarkdown {
  const headings: MarkdownHeading[] = []
  const markdown = createMarkdown(currentPath, headings)
  const rendered = markdown.render(source)
  const html = DOMPurify.sanitize(rendered, {
    USE_PROFILES: { html: true, svg: true, svgFilters: true, mathMl: true },
    ADD_ATTR: [
      'target',
      'rel',
      'role',
      'tabindex',
      'data-reader-path',
      'data-reader-hash',
      'data-invalid-source',
      'aria-hidden',
      'aria-label',
      'aria-checked',
    ],
    FORBID_TAGS: ['script', 'style', 'iframe', 'object', 'embed'],
  })
  return { html, headings }
}
