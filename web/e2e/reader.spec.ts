import AxeBuilder from '@axe-core/playwright'
import { expect, test, type Page } from '@playwright/test'

async function login(page: Page): Promise<void> {
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Web Reader' })).toBeVisible()
  await page.getByLabel('密码').fill('reader-test')
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await expect(page.getByRole('button', { name: '切换文件栏' })).toBeVisible()
}

async function openChapter(page: Page): Promise<void> {
  const fileToggle = page.getByRole('button', { name: '切换文件栏' })
  if ((await fileToggle.getAttribute('aria-expanded')) === 'false') await fileToggle.click()
  await page.getByRole('button', { name: 'book1' }).click()
  await page.getByRole('button', { name: 'chapter1.md' }).click()
  await expect(page.getByRole('heading', { name: 'Web Reader 验收文档' })).toBeVisible()
}

test('reads enhanced Markdown and persists the theme', async ({ page }) => {
  await login(page)
  await openChapter(page)

  const image = page.getByRole('img', { name: '相对路径图片' })
  await expect(image).toBeVisible()
  await expect
    .poll(() => image.evaluate((node: HTMLImageElement) => node.naturalWidth))
    .toBeGreaterThan(0)
  await expect(page.locator('.katex').first()).toBeVisible()
  await expect(page.locator('.mermaid-output svg')).toBeVisible({ timeout: 15_000 })
  await expect(page.locator('[role="checkbox"]')).toHaveCount(3)

  if (test.info().project.name === 'desktop-chromium') {
    await page.getByRole('button', { name: '夜间主题' }).click()
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'night')
    await page.reload()
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'night')
  }
})

test('meets the automated WCAG A and AA baseline', async ({ page }) => {
  await page.goto('/')
  const loginScan = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa']).analyze()
  expect(loginScan.violations).toEqual([])

  await login(page)
  await openChapter(page)
  const readerScan = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa'])
    .exclude('.mermaid-output')
    .analyze()
  expect(readerScan.violations).toEqual([])
})

test('keeps wide Mermaid diagrams readable with local scrolling', async ({ page }) => {
  await page.route('**/api/fs/text?*', async (route) => {
    const path = new URL(route.request().url()).searchParams.get('path')
    if (path !== 'book1/chapter1.md') {
      await route.continue()
      return
    }

    const content = `# Wide Mermaid

\`\`\`mermaid
flowchart LR
  A[Start node with a long label] --> B[Validate request payload]
  B --> C[Load workspace metadata]
  C --> D[Resolve the requested file path]
  D --> E[Read and sanitize Markdown]
  E --> F[Render a large Mermaid diagram]
  F --> G[Display without clipping]
  G --> H[Finish]
\`\`\``
    await route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({
        path,
        content,
        encoding: 'utf-8',
        size: content.length,
        modifiedAt: '2026-07-20T00:00:00Z',
      }),
    })
  })

  await login(page)
  const fileToggle = page.getByRole('button', { name: '切换文件栏' })
  if ((await fileToggle.getAttribute('aria-expanded')) === 'false') await fileToggle.click()
  await page.getByRole('button', { name: 'book1' }).click()
  await page.getByRole('button', { name: 'chapter1.md' }).click()
  await expect(page.getByRole('heading', { name: 'Wide Mermaid' })).toBeVisible()

  const svg = page.locator('.mermaid-output svg')
  await expect(svg).toBeVisible({ timeout: 15_000 })
  const layout = await svg.evaluate((node) => {
    const svgNode = node as SVGSVGElement
    const diagram = svgNode.closest<HTMLElement>('.mermaid-diagram')
    const edges = Array.from(svgNode.querySelectorAll<SVGPathElement>('.flowchart-link'))
    return {
      renderedWidth: svgNode.getBoundingClientRect().width,
      viewBoxWidth: svgNode.viewBox.baseVal.width,
      overflow: getComputedStyle(svgNode).overflow,
      clientWidth: diagram?.clientWidth ?? 0,
      scrollWidth: diagram?.scrollWidth ?? 0,
      edges: edges.map((edge) => ({
        length: edge.getTotalLength(),
        marker: edge.getAttribute('marker-end'),
      })),
    }
  })

  expect(layout.viewBoxWidth).toBeGreaterThan(1_500)
  expect(Math.abs(layout.renderedWidth - layout.viewBoxWidth)).toBeLessThan(1)
  expect(layout.scrollWidth).toBeGreaterThan(layout.clientWidth)
  expect(layout.overflow).toBe('visible')
  expect(layout.edges).toHaveLength(7)
  expect(layout.edges.every((edge) => edge.length > 0 && edge.marker?.startsWith('url(#'))).toBe(
    true,
  )
})

test('renders the Mermaid diagram gallery', async ({ page }) => {
  test.skip(
    !['desktop-chromium', 'desktop-firefox'].includes(test.info().project.name),
    'desktop gallery validation',
  )
  await login(page)

  await page.getByRole('button', { name: 'mermaid-gallery.md' }).click()
  await expect(page.getByRole('heading', { name: 'Mermaid 图表验收' })).toBeVisible()
  await expect(page.locator('.mermaid-output svg')).toHaveCount(12, { timeout: 30_000 })
  await expect(page.locator('.mermaid-error')).toHaveCount(0)
})

test('supports mobile drawers without page overflow', async ({ page }) => {
  test.skip(!test.info().project.name.startsWith('mobile-'), 'mobile interaction')
  await login(page)

  const fileToggle = page.getByRole('button', { name: '切换文件栏' })
  await fileToggle.click()
  await expect(page.getByRole('dialog', { name: '工作区文件' })).toBeVisible()
  await openChapter(page)
  await expect(fileToggle).toHaveAttribute('aria-expanded', 'false')

  const outlineToggle = page.getByRole('button', { name: '切换大纲栏' })
  await outlineToggle.click()
  await expect(page.getByRole('dialog', { name: '文章大纲' })).toBeVisible()
  await page.getByRole('button', { name: '公式', exact: true }).click()
  await expect(outlineToggle).toHaveAttribute('aria-expanded', 'false')

  const overflows = await page.evaluate(
    () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
  )
  expect(overflows).toBe(false)

  await page.getByRole('button', { name: '退出' }).click()
  await expect(page.getByRole('button', { name: '登录', exact: true })).toBeVisible()
})
