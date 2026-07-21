<script setup lang="ts">
import DOMPurify from 'dompurify'
import 'highlight.js/styles/github.css'
import 'katex/dist/katex.min.css'
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import panzoom, { type PanZoom } from 'panzoom'
import { ICON_PATHS } from '@/utils/icons'
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
const MAX_MERMAID_DIAGRAMS = 60
const MAX_MERMAID_SOURCE_LENGTH = 50_000

function getMermaidFontFamily(): string {
  return getComputedStyle(document.documentElement).getPropertyValue('--font-sans').trim() ||
    'ui-sans-serif, system-ui, sans-serif'
}

let mermaidRun = 0
let scrollFrame = 0
let scrollContainer: HTMLElement | null = null

const panzoomInstances = new Map<HTMLElement, PanZoom>()
const fullscreenMermaidHTML = ref<string>('')
const modalRotation = ref<number>(0)
const fullscreenOutputRef = ref<HTMLElement | null>(null)
let modalPanzoom: PanZoom | null = null

function cleanupPanzoom() {
  for (const pz of panzoomInstances.values()) {
    pz.dispose()
  }
  panzoomInstances.clear()
}

function handleZoomAction(pz: PanZoom, container: HTMLElement, action: string, isModal = false) {
  const rectWidth = container.clientWidth
  const rectHeight = container.clientHeight
  const cx = rectWidth / 2
  const cy = rectHeight / 2
  if (action === 'zoom-in') pz.smoothZoom(cx, cy, 1.2)
  else if (action === 'zoom-out') pz.smoothZoom(cx, cy, 1 / 1.2)
  else if (action === 'reset' || action === 'maximize') {
    if (!isModal) {
      pz.moveTo(0, 0)
      pz.zoomAbs(0, 0, 1)
    } else {
      const svg = container.querySelector('svg')
      if (svg) {
        let contentWidth = svg.clientWidth
        let contentHeight = svg.clientHeight
        
        if (modalRotation.value % 180 !== 0) {
          contentWidth = svg.clientHeight
          contentHeight = svg.clientWidth
        }
        
        if (contentWidth > 0 && contentHeight > 0) {
          const scaleX = rectWidth / contentWidth
          const scaleY = rectHeight / contentHeight
          let scale = Math.min(scaleX, scaleY) * 0.98
          
          pz.zoomAbs(0, 0, 1)
          pz.moveTo(0, 0)
          pz.zoomAbs(cx, cy, scale)
        }
      }
    }
  }
}

function closeFullscreen() {
  if (modalPanzoom) {
    modalPanzoom.dispose()
    modalPanzoom = null
  }
  fullscreenMermaidHTML.value = ''
  modalRotation.value = 0
}

function handleModalAction(action: string) {
  if (action === 'minimize' || action === 'close') closeFullscreen()
  else if (action === 'rotate') {
    modalRotation.value += 90
    if (modalPanzoom && fullscreenOutputRef.value) {
      handleZoomAction(modalPanzoom, fullscreenOutputRef.value, 'reset', true)
    }
  }
  else if (modalPanzoom && fullscreenOutputRef.value) {
    handleZoomAction(modalPanzoom, fullscreenOutputRef.value, action, true)
  }
}

watch(fullscreenOutputRef, (el) => {
  if (el) {
      modalPanzoom = panzoom(el, {
        maxZoom: 10,
        minZoom: 0.1,
        bounds: false
      })
      handleZoomAction(modalPanzoom, el, 'reset', true)
  }
})


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

function injectCodeCopyButtons(): void {
  if (!article.value) return
  for (const block of article.value.querySelectorAll<HTMLElement>('.code-block')) {
    if (block.querySelector('.code-copy-btn')) continue
    const btn = document.createElement('button')
    btn.className = 'code-copy-btn'
    btn.innerHTML = `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="display:inline-block;vertical-align:-0.15em;flex-shrink:0">${ICON_PATHS['clipboard']}</svg> 复制`
    btn.addEventListener('click', async () => {
      const code = block.querySelector('code')?.textContent ?? ''
      try {
        await navigator.clipboard.writeText(code)
        btn.classList.add('copied')
        btn.innerHTML = `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="display:inline-block;vertical-align:-0.15em;flex-shrink:0">${ICON_PATHS['check']}</svg> 已复制`
        setTimeout(() => {
          btn.classList.remove('copied')
          btn.innerHTML = `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="display:inline-block;vertical-align:-0.15em;flex-shrink:0">${ICON_PATHS['clipboard']}</svg> 复制`
        }, 2000)
      } catch {
        // clipboard unavailable
      }
    })
    block.appendChild(btn)
  }
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

function renderToolbarHTML(actions: string[]): string {
  return `<div class="mermaid-toolbar">${actions
    .map((a) => `<button class="mermaid-btn" data-action="${a}" title="${actionTitle(a)}"><svg viewBox="0 0 24 24">${ICON_PATHS[actionIcon(a)]}</svg></button>`)
    .join('')}</div>`
}

function actionTitle(action: string): string {
  const titles: Record<string, string> = {
    'zoom-in': 'Zoom In', 'zoom-out': 'Zoom Out', 'reset': 'Reset View',
    'maximize': 'Maximize', 'rotate': 'Rotate 90°', 'minimize': 'Minimize',
  }
  return titles[action] ?? action
}

function actionIcon(action: string): string {
  const map: Record<string, string> = {
    reset: 'rotate-ccw',
    rotate: 'rotate-cw',
    minimize: 'minimize',
  }
  return map[action] ?? action
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
      fontFamily: getMermaidFontFamily(),
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
        
        const toolbarHTML = renderToolbarHTML(['zoom-in', 'zoom-out', 'reset', 'maximize'])
        if (!diagram.querySelector('.mermaid-toolbar')) {
          diagram.insertAdjacentHTML('beforeend', toolbarHTML)
        }
        
        const svg = output.querySelector('svg')
        if (svg) {
          const pz = panzoom(svg, {
            maxZoom: 10,
            minZoom: 0.1,
            bounds: true,
            boundsPadding: 0.1
          })
          panzoomInstances.set(diagram, pz)
        }
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
  cleanupPanzoom()
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
  injectCodeCopyButtons()
  await renderMermaidDiagrams()
}

function handleClick(event: MouseEvent): void {
  const target = event.target
  if (!(target instanceof Element)) return

  const mermaidBtn = target.closest<HTMLButtonElement>('.mermaid-btn')
  const mermaidDiagram = target.closest<HTMLElement>('.mermaid-diagram')
  if (mermaidBtn && mermaidDiagram) {
    const action = mermaidBtn.dataset.action
    if (action === 'maximize') {
      const svgHTML = mermaidDiagram.querySelector('.mermaid-output')?.innerHTML
      if (svgHTML) {
        fullscreenMermaidHTML.value = svgHTML
      }
    } else if (action) {
      const pz = panzoomInstances.get(mermaidDiagram)
      if (pz) handleZoomAction(pz, mermaidDiagram, action, false)
    }
    return
  }

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
  cleanupPanzoom()
  closeFullscreen()
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

  <Teleport to="body">
    <dialog v-if="fullscreenMermaidHTML" class="mermaid-modal" open @close="closeFullscreen">
      <div class="mermaid-modal-backdrop" @click="closeFullscreen"></div>
      <div class="mermaid-modal-content">
        <div class="mermaid-toolbar modal-toolbar">
          <button class="mermaid-btn" data-action="zoom-in" title="Zoom In" @click="handleModalAction('zoom-in')"><svg viewBox="0 0 24 24" v-html="ICON_PATHS['zoom-in']"></svg></button>
          <button class="mermaid-btn" data-action="zoom-out" title="Zoom Out" @click="handleModalAction('zoom-out')"><svg viewBox="0 0 24 24" v-html="ICON_PATHS['zoom-out']"></svg></button>
          <button class="mermaid-btn" data-action="reset" title="Reset View" @click="handleModalAction('reset')"><svg viewBox="0 0 24 24" v-html="ICON_PATHS['rotate-ccw']"></svg></button>
          <button class="mermaid-btn" data-action="rotate" title="Rotate 90°" @click="handleModalAction('rotate')"><svg viewBox="0 0 24 24" v-html="ICON_PATHS['rotate-cw']"></svg></button>
          <button class="mermaid-btn" data-action="minimize" title="Minimize" @click="handleModalAction('minimize')"><svg viewBox="0 0 24 24" v-html="ICON_PATHS['minimize']"></svg></button>
        </div>
        <div class="mermaid-output fullscreen-output">
          <div ref="fullscreenOutputRef" class="panzoom-target">
            <div class="svg-rotator" :style="{ transform: 'rotate(' + modalRotation + 'deg)' }" v-html="fullscreenMermaidHTML"></div>
          </div>
        </div>
      </div>
    </dialog>
  </Teleport>
</template>

