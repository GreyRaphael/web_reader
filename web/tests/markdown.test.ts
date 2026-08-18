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

  it('rewrites local images and workspace file links', () => {
    const result = renderMarkdown(
      '![Cover](../images/cover.png)\n\n[Next](./next.md#details)\n\n[Config](./config.json)',
      'book/chapters/start.md',
    )
    const root = document.createElement('div')
    root.innerHTML = result.html

    expect(root.querySelector('img')?.getAttribute('src')).toBe(
      '/api/fs/raw?path=book%2Fimages%2Fcover.png',
    )
    const links = root.querySelectorAll('a')
    expect(links[0]?.dataset.readerPath).toBe('book/chapters/next.md')
    expect(links[0]?.dataset.readerHash).toBe('details')
    expect(links[1]?.dataset.readerPath).toBe('book/chapters/config.json')
  })

  it('removes escaping relative image sources', () => {
    const result = renderMarkdown('![No escape](../../secret.png)', 'guide/start.md')
    const root = document.createElement('div')
    root.innerHTML = result.html

    const image = root.querySelector('img')
    expect(image?.hasAttribute('src')).toBe(false)
    expect(image?.dataset.invalidSource).toBe('true')
  })

  it('renders task list markers as inert checkboxes', () => {
    const result = renderMarkdown('- [x] Complete\n- [ ] Pending', 'tasks.md')
    const root = document.createElement('div')
    root.innerHTML = result.html

    const checkboxes = root.querySelectorAll('[role="checkbox"]')
    expect(checkboxes).toHaveLength(2)
    expect(checkboxes[0]?.getAttribute('aria-checked')).toBe('true')
    expect(checkboxes[0]?.getAttribute('aria-label')).toBe('已完成')
    expect(checkboxes[1]?.getAttribute('aria-checked')).toBe('false')
    expect(checkboxes[1]?.getAttribute('aria-label')).toBe('未完成')
  })

  it('does not consume trailing text after a block math delimiter', () => {
    const result = renderMarkdown('$$x$$ trailing text', 'math.md')
    expect(result.html).toContain('trailing text')
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

  it('renders wiki-links [[Target]], [[Target|Alias]], [[Target#Heading]], and [[#Heading]]', () => {
    const result = renderMarkdown(
      'Check out [[Guide]] and [[Guide|使用指南]] as well as [[Architecture#Design|架构设计]] and [[#FAQ]].',
      'docs/index.md',
    )
    const root = document.createElement('div')
    root.innerHTML = result.html

    const links = root.querySelectorAll<HTMLAnchorElement>('a.wiki-link')
    expect(links).toHaveLength(4)

    expect(links[0]?.dataset.readerPath).toBe('docs/Guide.md')
    expect(links[0]?.textContent).toContain('Guide')

    expect(links[1]?.dataset.readerPath).toBe('docs/Guide.md')
    expect(links[1]?.textContent).toContain('使用指南')

    expect(links[2]?.dataset.readerPath).toBe('docs/Architecture.md')
    expect(links[2]?.dataset.readerHash).toBe('Design')
    expect(links[2]?.textContent).toContain('架构设计')

    expect(links[3]?.dataset.readerPath).toBe('docs/index.md')
    expect(links[3]?.dataset.readerHash).toBe('FAQ')
    expect(links[3]?.textContent).toContain('#FAQ')
  })

  it('renders file:// URI links as inert formatted spans with tooltip', () => {
    const result = renderMarkdown(
      'See [FileTree.vue](file:///home/gewei/workspace/web_reader/web/src/components/FileTree.vue) for details.',
      'docs/index.md',
    )
    const root = document.createElement('div')
    root.innerHTML = result.html

    expect(root.querySelector('a')).toBeNull()
    const span = root.querySelector<HTMLSpanElement>('span.file-uri-link')
    expect(span).not.toBeNull()
    expect(span?.textContent).toBe('FileTree.vue')
    expect(span?.getAttribute('title')).toBe(
      'file:///home/gewei/workspace/web_reader/web/src/components/FileTree.vue',
    )
    expect(span?.dataset.fileUri).toBe(
      'file:///home/gewei/workspace/web_reader/web/src/components/FileTree.vue',
    )
  })
})
