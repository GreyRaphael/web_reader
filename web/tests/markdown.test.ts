import { describe, expect, it } from 'vitest'
import { renderMarkdown } from '@/markdown/render'

describe('Markdown rendering', () => {
  it('renders all supported math delimiter forms', () => {
    const result = renderMarkdown(
      String.raw`Inline \(a+b\) and $c+d$.

$$
e=mc^2
$$

\[
x^2+y^2
\]`,
      'notes/math.md',
    )

    expect(result.html).toContain('class="math-inline"')
    expect(result.html.match(/class="math-block scroll-surface"/g)).toHaveLength(2)
    expect(result.html).toContain('class="katex"')
  })

  it('disables raw HTML and sanitizes dangerous link protocols', () => {
    const result = renderMarkdown(
      '<script>alert(1)</script>\n\n<img src=x onerror="alert(2)">\n\n[bad](javascript:alert(3))',
      'notes/security.md',
    )
    const root = document.createElement('div')
    root.innerHTML = result.html

    expect(root.querySelector('script')).toBeNull()
    expect(root.querySelector('img')).toBeNull()
    expect(root.querySelector('a')?.getAttribute('href') ?? null).toBeNull()
  })

  it('rewrites local images and readable links', () => {
    const result = renderMarkdown(
      '![Cover](../images/cover.png)\n\n[Next](./next.md#details)',
      'book/chapters/start.md',
    )
    const root = document.createElement('div')
    root.innerHTML = result.html

    expect(root.querySelector('img')?.getAttribute('src')).toBe(
      '/api/fs/raw?path=book%2Fimages%2Fcover.png',
    )
    expect(root.querySelector('a')?.dataset.readerPath).toBe('book/chapters/next.md')
    expect(root.querySelector('a')?.dataset.readerHash).toBe('details')
  })

  it('creates stable unique slugs for duplicate titles', () => {
    const result = renderMarkdown('# 快速开始\n\n## 快速开始\n\n## Hello, World!', 'readme.md')
    expect(result.headings).toEqual([
      { id: '快速开始', title: '快速开始', level: 1 },
      { id: '快速开始-1', title: '快速开始', level: 2 },
      { id: 'hello-world', title: 'Hello, World!', level: 2 },
    ])
    expect(result.html).toContain('id="快速开始-1"')
  })

  it('wraps wide content in local scroll surfaces', () => {
    const result = renderMarkdown(
      '| A | B |\n| - | - |\n| 1 | 2 |\n\n```ts\nconst value = 1\n```\n\n```mermaid\ngraph TD\nA-->B\n```',
      'readme.md',
    )
    expect(result.html).toContain('class="table-scroll scroll-surface"')
    expect(result.html).toContain('class="code-block scroll-surface"')
    expect(result.html).toContain('class="mermaid-diagram scroll-surface"')
  })
})
