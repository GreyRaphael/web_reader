<script setup lang="ts">
import DOMPurify from 'dompurify'
import 'katex/dist/katex.min.css'
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import panzoom, { type PanZoom } from 'panzoom'
import { saveTextFile } from '@/api/client'
import { ICON_PATHS } from '@/utils/icons'
import type { ResolvedTheme } from '@/composables/useTheme'
import { renderMarkdown, type MarkdownHeading, type RenderedMarkdown } from '@/markdown/render'
import { decodeFragment } from '@/utils/path'
import MarkdownEditor from './MarkdownEditor.vue'

type ViewMode = 'preview' | 'edit' | 'split'

const props = defineProps<{
  content: string
  currentPath: string
  theme: ResolvedTheme
}>()
const emit = defineEmits<{
  headings: [items: MarkdownHeading[]]
  activeHeading: [id: string]
  openPath: [path: string, hash: string]
  saved: [path: string]
}>()

const viewMode = ref<ViewMode>('preview')
const editableContent = ref(props.content)
const isSaving = ref(false)
const saveError = ref('')
const saveSuccess = ref(false)

const popoverText = ref('')
const popoverIsError = ref(false)
const popoverStyle = ref<{ top: string; left: string }>({ top: '0px', left: '0px' })
let popoverTimer: number | null = null

function showContextualToast(btn: HTMLButtonElement, message: string, isError = false) {
  const rect = btn.getBoundingClientRect()
  const top = Math.max(10, rect.top - 8)
  const left = rect.left + rect.width / 2

  popoverStyle.value = {
    top: `${top}px`,
    left: `${left}px`,
  }

  if (popoverTimer) clearTimeout(popoverTimer)
  popoverText.value = message
  popoverIsError.value = isError

  popoverTimer = window.setTimeout(() => {
    popoverText.value = ''
    popoverTimer = null
  }, 2000)
}

const isDirty = computed(() => editableContent.value !== props.content)

const editorRef = ref<InstanceType<typeof MarkdownEditor> | null>(null)
const previewColRef = ref<HTMLElement | null>(null)

let isSyncingEditor = false
let isSyncingPreview = false

function handleEditorScroll(e: Event) {
  if (viewMode.value !== 'split' || isSyncingPreview) return
  isSyncingEditor = true
  const target = e.target as HTMLElement
  const maxEditorScroll = target.scrollHeight - target.clientHeight
  if (maxEditorScroll <= 0) {
    isSyncingEditor = false
    return
  }
  const percentage = target.scrollTop / maxEditorScroll
  if (previewColRef.value) {
    const maxPreviewScroll = previewColRef.value.scrollHeight - previewColRef.value.clientHeight
    previewColRef.value.scrollTop = percentage * maxPreviewScroll
  }
  requestAnimationFrame(() => {
    isSyncingEditor = false
  })
}

function handlePreviewScroll(e: Event) {
  if (viewMode.value !== 'split' || isSyncingEditor) return
  isSyncingPreview = true
  const target = e.target as HTMLElement
  const maxPreviewScroll = target.scrollHeight - target.clientHeight
  if (maxPreviewScroll <= 0) {
    isSyncingPreview = false
    return
  }
  const percentage = target.scrollTop / maxPreviewScroll
  const textarea = editorRef.value?.getTextareaElement()
  if (textarea) {
    const maxEditorScroll = textarea.scrollHeight - textarea.clientHeight
    textarea.scrollTop = percentage * maxEditorScroll
    editorRef.value?.syncScroll()
  }
  requestAnimationFrame(() => {
    isSyncingPreview = false
  })
}

watch(
  () => props.content,
  (newVal) => {
    editableContent.value = newVal
  },
)

const article = ref<HTMLElement | null>(null)
const rendered = ref<RenderedMarkdown>({ html: '', headings: [] })
const renderError = ref('')
const MAX_MERMAID_DIAGRAMS = 60
const MAX_MERMAID_SOURCE_LENGTH = 50_000

function getMermaidFontFamily(): string {
  return (
    getComputedStyle(document.documentElement).getPropertyValue('--font-sans').trim() ||
    'ui-sans-serif, system-ui, sans-serif'
  )
}

let mermaidRun = 0
let scrollFrame = 0
let scrollContainer: HTMLElement | null = null

const panzoomInstances = new Map<HTMLElement, PanZoom>()
const fullscreenMermaidHTML = ref<string>('')
const fullscreenMermaidSource = ref<string>('')
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
          const scale = Math.min(scaleX, scaleY) * 0.98

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
  fullscreenMermaidSource.value = ''
  modalRotation.value = 0
}

async function handleModalAction(action: string, btnEvent?: MouseEvent) {
  const btn = btnEvent?.currentTarget as HTMLButtonElement | undefined
  if (action === 'minimize' || action === 'close') {
    closeFullscreen()
  } else if (action === 'rotate') {
    modalRotation.value += 90
    if (modalPanzoom && fullscreenOutputRef.value) {
      handleZoomAction(modalPanzoom, fullscreenOutputRef.value, 'reset', true)
    }
  } else if (action === 'copy-mermaid') {
    if (fullscreenMermaidSource.value) {
      navigator.clipboard.writeText(fullscreenMermaidSource.value).then(
        () => btn && showButtonFeedback(btn, '已复制 Mermaid 源码'),
        () => btn && showButtonFeedback(btn, '复制源码失败', true),
      )
    }
  } else if (action === 'copy-image') {
    const svg = fullscreenOutputRef.value?.querySelector('svg')
    if (svg) {
      svgToPngBlob(svg, { transparent: false }).then(
        async (blob) => {
          try {
            await navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })])
            btn && showButtonFeedback(btn, '已复制 PNG 图片 (白底)')
          } catch {
            btn && showButtonFeedback(btn, '剪切板写入受限', true)
          }
        },
        () => btn && showButtonFeedback(btn, '图片转换失败', true),
      )
    }
  } else if (action === 'export-image') {
    const svg = fullscreenOutputRef.value?.querySelector('svg')
    if (svg) {
      exportPngImage(svg, 'mermaid-diagram.png', { transparent: true }).then(
        () => btn && showButtonFeedback(btn, '已下载透明 PNG 图片'),
        () => btn && showButtonFeedback(btn, '导出 PNG 失败', true),
      )
    }
  } else if (modalPanzoom && fullscreenOutputRef.value) {
    handleZoomAction(modalPanzoom, fullscreenOutputRef.value, action, true)
  }
}

watch(fullscreenOutputRef, (el) => {
  if (el) {
    modalPanzoom = panzoom(el, {
      maxZoom: 10,
      minZoom: 0.1,
      bounds: false,
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
        showContextualToast(btn, '已复制代码块内容')
        setTimeout(() => {
          btn.classList.remove('copied')
          btn.innerHTML = `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="display:inline-block;vertical-align:-0.15em;flex-shrink:0">${ICON_PATHS['clipboard']}</svg> 复制`
        }, 2000)
      } catch {
        showContextualToast(btn, '代码复制失败', true)
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
    .map(
      (a) =>
        `<button class="mermaid-btn" data-action="${a}" title="${actionTitle(a)}"><svg viewBox="0 0 24 24">${ICON_PATHS[actionIcon(a)]}</svg></button>`,
    )
    .join('')}</div>`
}

function actionTitle(action: string): string {
  const titles: Record<string, string> = {
    'zoom-in': '放大',
    'zoom-out': '缩小',
    reset: '重置视角',
    'copy-mermaid': '复制 Mermaid 源码',
    'copy-image': '复制 PNG 图片 (白底)',
    'export-image': '导出透明 PNG 图片',
    maximize: '全屏查看',
    rotate: '旋转 90°',
    minimize: '关闭全屏',
  }
  return titles[action] ?? action
}

function actionIcon(action: string): string {
  const map: Record<string, string> = {
    reset: 'rotate-ccw',
    rotate: 'rotate-cw',
    minimize: 'minimize',
    'copy-mermaid': 'file-code',
    'copy-image': 'copy',
    'export-image': 'download',
  }
  return map[action] ?? action
}

async function svgToPngBlob(
  svgElement: SVGSVGElement,
  options: { transparent?: boolean } = {},
): Promise<Blob> {
  return new Promise((resolve, reject) => {
    const clone = svgElement.cloneNode(true) as SVGSVGElement
    const viewBox = svgElement.viewBox.baseVal
    const width = viewBox && viewBox.width > 0 ? viewBox.width : svgElement.clientWidth || 800
    const height = viewBox && viewBox.height > 0 ? viewBox.height : svgElement.clientHeight || 600

    clone.setAttribute('width', `${width}`)
    clone.setAttribute('height', `${height}`)

    const svgString = new XMLSerializer().serializeToString(clone)
    const svgBlob = new Blob([svgString], { type: 'image/svg+xml;charset=utf-8' })
    const url = URL.createObjectURL(svgBlob)

    const img = new Image()
    img.crossOrigin = 'anonymous'
    img.onload = () => {
      const canvas = document.createElement('canvas')
      const scale = 2
      canvas.width = width * scale
      canvas.height = height * scale
      const ctx = canvas.getContext('2d')
      if (!ctx) {
        URL.revokeObjectURL(url)
        reject(new Error('Canvas 不可用'))
        return
      }
      if (!options.transparent) {
        ctx.fillStyle = '#ffffff'
        ctx.fillRect(0, 0, canvas.width, canvas.height)
      } else {
        ctx.clearRect(0, 0, canvas.width, canvas.height)
      }
      ctx.scale(scale, scale)
      ctx.drawImage(img, 0, 0, width, height)
      URL.revokeObjectURL(url)

      canvas.toBlob((blob) => {
        if (blob) resolve(blob)
        else reject(new Error('转换 Blob 失败'))
      }, 'image/png')
    }
    img.onerror = (err) => {
      URL.revokeObjectURL(url)
      reject(err)
    }
    img.src = url
  })
}

async function exportPngImage(
  svgElement: SVGSVGElement,
  filename = 'mermaid-diagram.png',
  options: { transparent?: boolean } = { transparent: true },
): Promise<void> {
  const blob = await svgToPngBlob(svgElement, options)
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

function showButtonFeedback(btn: HTMLButtonElement, successMessage: string, isError = false) {
  const originalTitle = btn.getAttribute('title') || ''
  btn.setAttribute('title', successMessage)
  btn.style.color = isError ? 'var(--danger)' : 'var(--accent-strong)'
  showContextualToast(btn, successMessage, isError)
  setTimeout(() => {
    btn.setAttribute('title', originalTitle)
    btn.style.color = ''
  }, 2000)
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

        const toolbarHTML = renderToolbarHTML([
          'zoom-in',
          'zoom-out',
          'reset',
          'copy-mermaid',
          'copy-image',
          'export-image',
          'maximize',
        ])
        if (!diagram.querySelector('.mermaid-toolbar')) {
          diagram.insertAdjacentHTML('beforeend', toolbarHTML)
        }

        const svg = output.querySelector('svg')
        if (svg) {
          const pz = panzoom(svg, {
            maxZoom: 10,
            minZoom: 0.1,
            bounds: true,
            boundsPadding: 0.1,
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

async function updateMarkdownText(sourceText: string): Promise<void> {
  cleanupPanzoom()
  try {
    rendered.value = renderMarkdown(sourceText, props.currentPath)
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

async function handleSave(): Promise<void> {
  if (!isDirty.value || isSaving.value) return
  isSaving.value = true
  saveError.value = ''
  try {
    await saveTextFile(props.currentPath, editableContent.value)
    saveSuccess.value = true
    emit('saved', props.currentPath)
    setTimeout(() => {
      saveSuccess.value = false
    }, 2000)
  } catch (err) {
    saveError.value = err instanceof Error ? err.message : '保存失败'
  } finally {
    isSaving.value = false
  }
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
      const sourceCode = mermaidDiagram.querySelector<HTMLElement>('.mermaid-source')?.textContent ?? ''
      if (svgHTML) {
        fullscreenMermaidHTML.value = svgHTML
        fullscreenMermaidSource.value = sourceCode
      }
    } else if (action === 'copy-mermaid') {
      const source = mermaidDiagram.querySelector<HTMLElement>('.mermaid-source')?.textContent ?? ''
      if (source) {
        navigator.clipboard.writeText(source).then(
          () => showButtonFeedback(mermaidBtn, '已复制 Mermaid 源码'),
          () => showButtonFeedback(mermaidBtn, '复制源码失败', true),
        )
      }
    } else if (action === 'copy-image') {
      const svg = mermaidDiagram.querySelector<SVGSVGElement>('.mermaid-output svg')
      if (svg) {
        svgToPngBlob(svg, { transparent: false }).then(
          async (blob) => {
            try {
              await navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })])
              showButtonFeedback(mermaidBtn, '已复制 PNG 图片 (白底)')
            } catch {
              showButtonFeedback(mermaidBtn, '剪切板写入受限', true)
            }
          },
          () => showButtonFeedback(mermaidBtn, '图片转换失败', true),
        )
      }
    } else if (action === 'export-image') {
      const svg = mermaidDiagram.querySelector<SVGSVGElement>('.mermaid-output svg')
      if (svg) {
        exportPngImage(svg, 'mermaid-diagram.png', { transparent: true }).then(
          () => showButtonFeedback(mermaidBtn, '已下载透明 PNG 图片'),
          () => showButtonFeedback(mermaidBtn, '导出 PNG 失败', true),
        )
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

watch(
  () => [props.content, props.currentPath],
  () => {
    updateMarkdownText(editableContent.value)
  },
  { immediate: true },
)

watch(editableContent, (newText) => {
  if (viewMode.value !== 'edit') {
    updateMarkdownText(newText)
  }
})

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
  <div class="markdown-container">
    <div class="markdown-toolbar">
      <div class="mode-switch-group">
        <button
          type="button"
          class="mode-btn"
          :class="{ active: viewMode === 'preview' }"
          @click="viewMode = 'preview'"
        >
          👁 预览
        </button>
        <button
          type="button"
          class="mode-btn"
          :class="{ active: viewMode === 'edit' }"
          @click="viewMode = 'edit'"
        >
          ✏️ 编辑
        </button>
        <button
          type="button"
          class="mode-btn"
          :class="{ active: viewMode === 'split' }"
          @click="viewMode = 'split'"
        >
          📑 分屏
        </button>
      </div>

      <div class="markdown-toolbar-actions">
        <span v-if="isDirty" class="dirty-tag">* 已修改</span>
        <span v-if="saveSuccess" class="success-tag">✓ 已保存</span>
        <span v-if="saveError" class="error-tag">{{ saveError }}</span>
        <button
          type="button"
          class="save-btn"
          :disabled="!isDirty || isSaving"
          @click="handleSave"
        >
          {{ isSaving ? '保存中…' : '保存' }}
        </button>
      </div>
    </div>

    <div class="markdown-body-wrapper" :class="`mode-${viewMode}`">
      <div v-if="viewMode === 'edit' || viewMode === 'split'" class="editor-pane">
        <MarkdownEditor
          ref="editorRef"
          v-model:content="editableContent"
          @save="handleSave"
          @scroll="handleEditorScroll"
        />
      </div>

      <div
        v-if="viewMode === 'preview' || viewMode === 'split'"
        ref="previewColRef"
        class="preview-pane-col"
        @scroll="handlePreviewScroll"
      >
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
      </div>
    </div>

    <Teleport to="body">
      <Transition name="popover-toast">
        <div
          v-if="popoverText"
          class="mermaid-popover-toast"
          :class="{ error: popoverIsError }"
          :style="popoverStyle"
          role="status"
        >
          <span>{{ popoverText }}</span>
          <div class="popover-arrow"></div>
        </div>
      </Transition>

      <dialog v-if="fullscreenMermaidHTML" class="mermaid-modal" open @close="closeFullscreen">
        <div class="mermaid-modal-backdrop" @click="closeFullscreen"></div>
        <div class="mermaid-modal-content">
          <div class="mermaid-toolbar modal-toolbar">
            <button
              class="mermaid-btn"
              data-action="zoom-in"
              title="放大"
              @click="handleModalAction('zoom-in', $event)"
            >
              <svg viewBox="0 0 24 24" v-html="ICON_PATHS['zoom-in']"></svg>
            </button>
            <button
              class="mermaid-btn"
              data-action="zoom-out"
              title="缩小"
              @click="handleModalAction('zoom-out', $event)"
            >
              <svg viewBox="0 0 24 24" v-html="ICON_PATHS['zoom-out']"></svg>
            </button>
            <button
              class="mermaid-btn"
              data-action="reset"
              title="重置视角"
              @click="handleModalAction('reset', $event)"
            >
              <svg viewBox="0 0 24 24" v-html="ICON_PATHS['rotate-ccw']"></svg>
            </button>
            <button
              class="mermaid-btn"
              data-action="copy-mermaid"
              title="复制 Mermaid 源码"
              @click="handleModalAction('copy-mermaid', $event)"
            >
              <svg viewBox="0 0 24 24" v-html="ICON_PATHS['file-code']"></svg>
            </button>
            <button
              class="mermaid-btn"
              data-action="copy-image"
              title="复制 PNG 图片 (白底)"
              @click="handleModalAction('copy-image', $event)"
            >
              <svg viewBox="0 0 24 24" v-html="ICON_PATHS['copy']"></svg>
            </button>
            <button
              class="mermaid-btn"
              data-action="export-image"
              title="导出透明 PNG 图片"
              @click="handleModalAction('export-image', $event)"
            >
              <svg viewBox="0 0 24 24" v-html="ICON_PATHS['download']"></svg>
            </button>
            <button
              class="mermaid-btn"
              data-action="rotate"
              title="旋转 90°"
              @click="handleModalAction('rotate', $event)"
            >
              <svg viewBox="0 0 24 24" v-html="ICON_PATHS['rotate-cw']"></svg>
            </button>
            <button
              class="mermaid-btn"
              data-action="minimize"
              title="关闭全屏"
              @click="handleModalAction('minimize', $event)"
            >
              <svg viewBox="0 0 24 24" v-html="ICON_PATHS['minimize']"></svg>
            </button>
          </div>
          <div class="mermaid-output fullscreen-output">
            <div ref="fullscreenOutputRef" class="panzoom-target">
              <div
                class="svg-rotator"
                :style="{ transform: 'rotate(' + modalRotation + 'deg)' }"
                v-html="fullscreenMermaidHTML"
              ></div>
            </div>
          </div>
        </div>
      </dialog>
    </Teleport>
  </div>
</template>

<style scoped>
.markdown-container {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
}

.markdown-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
  border-bottom: 1px solid var(--border);
  background: var(--surface-raised);
}

.mode-switch-group {
  display: flex;
  gap: 4px;
  padding: 3px;
  border: 1px solid var(--border);
  border-radius: 7px;
  background: var(--surface-muted);
}

.mode-btn {
  padding: 3px 10px;
  border: none;
  border-radius: 5px;
  background: transparent;
  color: var(--text-muted);
  font-family: var(--font-sans);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition:
    background-color 120ms ease,
    color 120ms ease;
}

.mode-btn:hover {
  color: var(--text);
}

.mode-btn.active {
  background: var(--surface);
  color: var(--accent-strong);
  font-weight: 700;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
}

.markdown-toolbar-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.dirty-tag {
  color: var(--accent-strong);
  font-family: var(--font-sans);
  font-size: 12px;
  font-weight: 600;
}

.success-tag {
  color: #2e7d32;
  font-family: var(--font-sans);
  font-size: 12px;
  font-weight: 600;
}

.error-tag {
  color: var(--danger);
  font-family: var(--font-sans);
  font-size: 12px;
}

.save-btn {
  padding: 4px 12px;
  border: 1px solid var(--accent);
  border-radius: 6px;
  background: var(--accent);
  color: #fff;
  font-family: var(--font-sans);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: opacity 120ms ease;
}

.save-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
  border-color: var(--border);
  background: var(--surface-muted);
  color: var(--text-faint);
}

.save-btn:not(:disabled):hover {
  opacity: 0.9;
}

.markdown-body-wrapper {
  display: flex;
  flex: 1;
  width: 100%;
  min-height: 0;
}

.markdown-body-wrapper.mode-preview .preview-pane-col,
.markdown-body-wrapper.mode-edit .editor-pane {
  width: 100%;
  height: 100%;
}

.markdown-body-wrapper.mode-split {
  gap: 16px;
  padding: 16px;
}

.markdown-body-wrapper.mode-split .editor-pane,
.markdown-body-wrapper.mode-split .preview-pane-col {
  flex: 1;
  width: 50%;
  height: 100%;
  min-width: 0;
  overflow-y: auto;
}

.editor-pane {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.preview-pane-col {
  height: 100%;
  overflow-y: auto;
}

.mermaid-popover-toast {
  position: fixed;
  z-index: 100000;
  transform: translate(-50%, -100%);
  padding: 5px 10px;
  border-radius: 6px;
  background: #1e1e2e;
  color: #ffffff;
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.25);
  font-family: var(--font-sans);
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
  pointer-events: none;
}

.mermaid-popover-toast.error {
  background: var(--danger);
  color: #ffffff;
}

.popover-arrow {
  position: absolute;
  bottom: -4px;
  left: 50%;
  transform: translateX(-50%) rotate(45deg);
  width: 8px;
  height: 8px;
  background: inherit;
}

.popover-toast-enter-active,
.popover-toast-leave-active {
  transition:
    opacity 150ms ease,
    transform 150ms ease;
}

.popover-toast-enter-from,
.popover-toast-leave-to {
  opacity: 0;
  transform: translate(-50%, -85%) scale(0.92);
}
</style>
