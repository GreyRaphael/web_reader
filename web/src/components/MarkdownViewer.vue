<script setup lang="ts">
import DOMPurify from 'dompurify'
import 'highlight.js/styles/github.css'
import 'katex/dist/katex.min.css'
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import type { ResolvedTheme } from '@/composables/useTheme'
import { renderMarkdown, type MarkdownHeading, type RenderedMarkdown } from '@/markdown/render'
import { decodeFragment } from '@/utils/path'

const props = defineProps<{
  content: string
  currentPath: string
  theme: ResolvedTheme
}>()
const emit = defineEmits<{
  headings: [items: MarkdownHeading[]]
  activeHeading: [id: string]
  openPath: [path: string, hash: string]
}>()

const article = ref<HTMLElement | null>(null)
const rendered = ref<RenderedMarkdown>({ html: '', headings: [] })
const renderError = ref('')
const MAX_MERMAID_DIAGRAMS = 20
const MAX_MERMAID_SOURCE_LENGTH = 50_000
const MERMAID_FONT_FAMILY =
  'Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif'

let mermaidRun = 0
let scrollFrame = 0
let scrollContainer: HTMLElement | null = null

function removeScrollSpy(): void {
  if (scrollContainer) scrollContainer.removeEventListener('scroll', requestScrollUpdate)
  scrollContainer = null
  if (scrollFrame) cancelAnimationFrame(scrollFrame)
  scrollFrame = 0
}

function updateActiveHeading(): void {
  scrollFrame = 0
  if (!article.value || !scrollContainer) return
  const headings = Array.from(
    article.value.querySelectorAll<HTMLElement>('h1[id], h2[id], h3[id], h4[id], h5[id], h6[id]'),
  )
  if (headings.length === 0) {
    emit('activeHeading', '')
    return
  }
  const containerTop = scrollContainer.getBoundingClientRect().top
  let active = headings[0]?.id ?? ''
  for (const heading of headings) {
    if (heading.getBoundingClientRect().top - containerTop <= 120) active = heading.id
    else break
  }
  emit('activeHeading', active)
}

function requestScrollUpdate(): void {
  if (!scrollFrame) scrollFrame = requestAnimationFrame(updateActiveHeading)
}

function installScrollSpy(): void {
  removeScrollSpy()
  scrollContainer = article.value?.closest<HTMLElement>('.preview-scroll') ?? null
  scrollContainer?.addEventListener('scroll', requestScrollUpdate, { passive: true })
  requestScrollUpdate()
}

function showMermaidError(diagram: HTMLElement, message: string): void {
  const output = diagram.querySelector<HTMLElement>('.mermaid-output')
  if (!output) return
  output.removeAttribute('aria-busy')
  output.classList.add('mermaid-error')
  output.textContent = message
}

function preserveMermaidSize(output: HTMLElement): void {
  const svg = output.querySelector<SVGSVGElement>('svg')
  const viewBox = svg?.viewBox.baseVal
  if (!svg || !viewBox || viewBox.width <= 0 || viewBox.height <= 0) return

  const naturalWidth = viewBox.width
  svg.style.width = '100%'
  svg.style.maxWidth = `${naturalWidth}px`
  svg.style.height = 'auto'
}

async function renderMermaidDiagrams(): Promise<void> {
  const run = ++mermaidRun
  const root = article.value
  if (!root) return
  const allDiagrams = Array.from(root.querySelectorAll<HTMLElement>('.mermaid-diagram'))
  if (allDiagrams.length === 0) return

  for (const diagram of allDiagrams.slice(MAX_MERMAID_DIAGRAMS)) {
    showMermaidError(diagram, `图表数量超过每篇文档 ${MAX_MERMAID_DIAGRAMS} 个的限制`)
  }
  const diagrams = allDiagrams.slice(0, MAX_MERMAID_DIAGRAMS).filter((diagram) => {
    const source = diagram.querySelector<HTMLElement>('.mermaid-source')?.textContent ?? ''
    if (source.length <= MAX_MERMAID_SOURCE_LENGTH) return true
    showMermaidError(
      diagram,
      `图表源码超过 ${MAX_MERMAID_SOURCE_LENGTH.toLocaleString()} 个字符的限制`,
    )
    return false
  })
  if (diagrams.length === 0) return

  try {
    const module = await import('mermaid')
    if (run !== mermaidRun) return
    const mermaid = module.default
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: 'strict',
      theme: props.theme === 'night' ? 'dark' : 'default',
      htmlLabels: false,
      fontFamily: MERMAID_FONT_FAMILY,
      suppressErrorRendering: true,
    })

    for (const [index, diagram] of diagrams.entries()) {
      if (run !== mermaidRun || !diagram.isConnected) return
      const source = diagram.querySelector<HTMLElement>('.mermaid-source')?.textContent ?? ''
      const output = diagram.querySelector<HTMLElement>('.mermaid-output')
      if (!output || !source.trim()) continue
      output.classList.remove('mermaid-error')
      output.replaceChildren()
      output.setAttribute('aria-busy', 'true')
      try {
        const id = `reader-mermaid-${run}-${index}`
        const result = await mermaid.render(id, source)
        if (run !== mermaidRun || !diagram.isConnected) return
        output.innerHTML = DOMPurify.sanitize(result.svg, {
          USE_PROFILES: { svg: true, svgFilters: true },
          ADD_TAGS: ['style'],
          ADD_ATTR: ['dominant-baseline'],
          FORBID_TAGS: ['script', 'foreignObject', 'iframe', 'object', 'embed'],
        })
        preserveMermaidSize(output)
      } catch (error) {
        if (run !== mermaidRun) return
        output.classList.add('mermaid-error')
        output.textContent = `图表渲染失败：${error instanceof Error ? error.message : '语法错误'}`
      } finally {
        output.removeAttribute('aria-busy')
      }
    }
  } catch (error) {
    if (run !== mermaidRun) return
    for (const output of root.querySelectorAll<HTMLElement>('.mermaid-output')) {
      output.classList.add('mermaid-error')
      output.textContent = `图表模块加载失败：${error instanceof Error ? error.message : '未知错误'}`
    }
  }
  requestScrollUpdate()
}

async function updateMarkdown(): Promise<void> {
  try {
    rendered.value = renderMarkdown(props.content, props.currentPath)
    renderError.value = ''
    emit('headings', rendered.value.headings)
  } catch (error) {
    renderError.value = error instanceof Error ? error.message : 'Markdown 渲染失败'
    rendered.value = { html: '', headings: [] }
    emit('headings', [])
  }
  await nextTick()
  installScrollSpy()
  await renderMermaidDiagrams()
}

function handleClick(event: MouseEvent): void {
  const target = event.target
  if (!(target instanceof Element)) return
  const anchor = target.closest<HTMLAnchorElement>('a')
  if (!anchor || !article.value?.contains(anchor)) return

  const path = anchor.dataset.readerPath
  if (path) {
    event.preventDefault()
    emit('openPath', path, anchor.dataset.readerHash ?? '')
    return
  }
  const href = anchor.getAttribute('href') ?? ''
  if (href.startsWith('#')) {
    event.preventDefault()
    const id = decodeFragment(href.slice(1))
    scrollToHeading(id)
    const url = new URL(window.location.href)
    url.hash = encodeURIComponent(id)
    window.history.pushState({}, '', url)
  }
}

function scrollToHeading(id: string): void {
  if (!article.value) return
  const heading = Array.from(article.value.querySelectorAll<HTMLElement>('[id]')).find(
    (node) => node.id === id,
  )
  heading?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

defineExpose({ scrollToHeading })

watch(() => [props.content, props.currentPath], updateMarkdown, { immediate: true })
watch(
  () => props.theme,
  async () => {
    await nextTick()
    await renderMermaidDiagrams()
  },
)

onBeforeUnmount(() => {
  mermaidRun += 1
  removeScrollSpy()
})
</script>

<template>
  <article
    v-if="!renderError"
    ref="article"
    class="markdown-body"
    aria-label="Markdown 内容"
    @click="handleClick"
    @load.capture="requestScrollUpdate"
    v-html="rendered.html"
  ></article>
  <div v-else class="preview-state error" role="alert">
    <div class="state-icon">!</div>
    <h2>Markdown 无法显示</h2>
    <p>{{ renderError }}</p>
  </div>
</template>
