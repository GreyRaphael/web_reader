import type MarkdownIt from 'markdown-it'
import katex from 'katex'

function renderMath(content: string, displayMode: boolean): string {
  try {
    return katex.renderToString(content, {
      displayMode,
      throwOnError: false,
      strict: 'ignore',
      trust: false,
      output: 'htmlAndMathml',
    })
  } catch (error) {
    const message = error instanceof Error ? error.message : '公式渲染失败'
    return `<code class="math-error">${escapeHtml(message)}</code>`
  }
}

function escapeHtml(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#039;')
}

export function mathPlugin(md: MarkdownIt): void {
  md.block.ruler.before('fence', 'math_block', (state, startLine, endLine, silent) => {
    const start = (state.bMarks[startLine] ?? 0) + (state.tShift[startLine] ?? 0)
    const max = state.eMarks[startLine]
    const line = state.src.slice(start, max).trim()
    const opening = line.startsWith('$$') ? '$$' : line.startsWith('\\[') ? '\\[' : ''
    if (!opening) return false

    const closing = opening === '$$' ? '$$' : '\\]'
    const firstContent = line.slice(opening.length)
    const sameLineClose = firstContent.indexOf(closing)
    if (sameLineClose >= 0) {
      if (firstContent.slice(sameLineClose + closing.length).trim()) return false
      if (silent) return true
      const token = state.push('math_block', 'math', 0)
      token.block = true
      token.content = firstContent.slice(0, sameLineClose).trim()
      token.map = [startLine, startLine + 1]
      state.line = startLine + 1
      return true
    }

    const content: string[] = []
    if (firstContent) content.push(firstContent)
    let nextLine = startLine + 1
    let found = false
    for (; nextLine < endLine; nextLine += 1) {
      const lineStart = (state.bMarks[nextLine] ?? 0) + (state.tShift[nextLine] ?? 0)
      const lineEnd = state.eMarks[nextLine]
      const current = state.src.slice(lineStart, lineEnd)
      const closeIndex = current.indexOf(closing)
      if (closeIndex >= 0) {
        if (current.slice(closeIndex + closing.length).trim()) return false
        content.push(current.slice(0, closeIndex))
        found = true
        break
      }
      content.push(current)
    }
    if (!found) return false
    if (silent) return true

    const token = state.push('math_block', 'math', 0)
    token.block = true
    token.content = content.join('\n').trim()
    token.map = [startLine, nextLine + 1]
    state.line = nextLine + 1
    return true
  })

  md.inline.ruler.before('escape', 'math_inline_paren', (state, silent) => {
    const start = state.pos
    if (!state.src.startsWith('\\(', start)) return false
    const end = state.src.indexOf('\\)', start + 2)
    if (end < 0 || end === start + 2) return false
    if (!silent) {
      const token = state.push('math_inline', 'math', 0)
      token.content = state.src.slice(start + 2, end)
    }
    state.pos = end + 2
    return true
  })

  md.inline.ruler.before('escape', 'math_inline_dollar', (state, silent) => {
    const start = state.pos
    if (state.src[start] !== '$') return false

    const double = state.src[start + 1] === '$'
    const marker = double ? '$$' : '$'
    const contentStart = start + marker.length
    let end = contentStart
    while (end < state.posMax) {
      end = state.src.indexOf(marker, end)
      if (end < 0) return false
      if (state.src[end - 1] !== '\\') break
      end += marker.length
    }
    if (end <= contentStart) return false

    const content = state.src.slice(contentStart, end)
    if (!double && (/^\s/.test(content) || /\s$/.test(content))) return false
    if (!silent) {
      const token = state.push(double ? 'math_inline_display' : 'math_inline', 'math', 0)
      token.content = content
    }
    state.pos = end + marker.length
    return true
  })

  md.renderer.rules.math_inline = (tokens, index) =>
    `<span class="math-inline">${renderMath(tokens[index]?.content ?? '', false)}</span>`
  md.renderer.rules.math_inline_display = (tokens, index) =>
    `<span class="math-display-inline scroll-surface" tabindex="0">${renderMath(tokens[index]?.content ?? '', true)}</span>`
  md.renderer.rules.math_block = (tokens, index) =>
    `<div class="math-block scroll-surface" tabindex="0">${renderMath(tokens[index]?.content ?? '', true)}</div>\n`
}
