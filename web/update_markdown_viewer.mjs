import fs from 'fs'

const path = '/home/gewei/workspace/web_reader/web/src/components/MarkdownViewer.vue'
let content = fs.readFileSync(path, 'utf8')

// Add imports
content = content.replace(
  "import { nextTick, onBeforeUnmount, ref, watch } from 'vue'",
  "import { nextTick, onBeforeUnmount, ref, watch, shallowRef } from 'vue'\nimport panzoom, { type PanZoom } from 'panzoom'\nimport { ICON_PATHS } from '@/utils/icons'"
)

// Add panzoom instances
const panzoomCode = `
const panzoomInstances = new Map<HTMLElement, PanZoom>()
const fullscreenMermaidHTML = ref<string>('')
const fullscreenOutputRef = ref<HTMLElement | null>(null)
let modalPanzoom: PanZoom | null = null

function cleanupPanzoom() {
  for (const pz of panzoomInstances.values()) {
    pz.dispose()
  }
  panzoomInstances.clear()
}

function handleZoomAction(pz: PanZoom, container: HTMLElement, action: string) {
  const rect = container.getBoundingClientRect()
  const cx = rect.width / 2
  const cy = rect.height / 2
  if (action === 'zoom-in') pz.smoothZoom(cx, cy, 1.5)
  else if (action === 'zoom-out') pz.smoothZoom(cx, cy, 1 / 1.5)
  else if (action === 'reset' || action === 'fit') {
    pz.moveTo(0, 0)
    pz.zoomAbs(0, 0, 1)
  }
}

function closeFullscreen() {
  if (modalPanzoom) {
    modalPanzoom.dispose()
    modalPanzoom = null
  }
  fullscreenMermaidHTML.value = ''
}

function handleModalAction(action: string) {
  if (action === 'close') closeFullscreen()
  else if (modalPanzoom && fullscreenOutputRef.value) {
    handleZoomAction(modalPanzoom, fullscreenOutputRef.value, action)
  }
}

watch(fullscreenOutputRef, (el) => {
  if (el) {
    const svg = el.querySelector('svg')
    if (svg) {
      modalPanzoom = panzoom(svg, {
        maxZoom: 10,
        minZoom: 0.1,
        bounds: true,
        boundsPadding: 0.1,
      })
    }
  }
})
`
content = content.replace('let scrollContainer: HTMLElement | null = null', 'let scrollContainer: HTMLElement | null = null\n' + panzoomCode)

// Insert toolbar rendering and panzoom initialization
content = content.replace(
  'preserveMermaidSize(output)',
  `preserveMermaidSize(output)
        
        const toolbarHTML = \`
          <div class="mermaid-toolbar">
            <button class="mermaid-btn" data-action="zoom-in" title="Zoom In"><svg viewBox="0 0 24 24">\${ICON_PATHS['zoom-in']}</svg></button>
            <button class="mermaid-btn" data-action="zoom-out" title="Zoom Out"><svg viewBox="0 0 24 24">\${ICON_PATHS['zoom-out']}</svg></button>
            <button class="mermaid-btn" data-action="reset" title="Reset View"><svg viewBox="0 0 24 24">\${ICON_PATHS['rotate-ccw']}</svg></button>
            <button class="mermaid-btn" data-action="fit" title="Fit to Screen"><svg viewBox="0 0 24 24">\${ICON_PATHS['maximize']}</svg></button>
          </div>
        \`
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
        }`
)

// Update handleClick to catch toolbar events
const handleClickCode = `
  const mermaidBtn = target.closest<HTMLButtonElement>('.mermaid-btn')
  const mermaidDiagram = target.closest<HTMLElement>('.mermaid-diagram')
  if (mermaidBtn && mermaidDiagram) {
    const action = mermaidBtn.dataset.action
    if (action) {
      const pz = panzoomInstances.get(mermaidDiagram)
      if (pz) handleZoomAction(pz, mermaidDiagram, action)
    }
    return
  }
  
  if (mermaidDiagram && !mermaidBtn) {
    const svgHTML = mermaidDiagram.querySelector('.mermaid-output')?.innerHTML
    if (svgHTML) {
      fullscreenMermaidHTML.value = svgHTML
    }
    return
  }

  const anchor = target.closest<HTMLAnchorElement>('a')
`
content = content.replace("  const anchor = target.closest<HTMLAnchorElement>('a')", handleClickCode)

// Clean up panzoom instances on updateMarkdown
content = content.replace(
  'try {\n    rendered.value = renderMarkdown(props.content, props.currentPath)',
  'cleanupPanzoom()\n  try {\n    rendered.value = renderMarkdown(props.content, props.currentPath)'
)

// Clean up on unmount
content = content.replace(
  'removeScrollSpy()',
  'removeScrollSpy()\n  cleanupPanzoom()\n  closeFullscreen()'
)

// Add dialog template
const dialogTemplate = `
  <Teleport to="body">
    <dialog v-if="fullscreenMermaidHTML" class="mermaid-modal" open @close="closeFullscreen">
      <div class="mermaid-modal-backdrop" @click="closeFullscreen"></div>
      <div class="mermaid-modal-content">
        <div class="mermaid-toolbar modal-toolbar">
          <button class="mermaid-btn" data-action="zoom-in" title="Zoom In" @click="handleModalAction('zoom-in')"><svg viewBox="0 0 24 24" v-html="ICON_PATHS['zoom-in']"></svg></button>
          <button class="mermaid-btn" data-action="zoom-out" title="Zoom Out" @click="handleModalAction('zoom-out')"><svg viewBox="0 0 24 24" v-html="ICON_PATHS['zoom-out']"></svg></button>
          <button class="mermaid-btn" data-action="reset" title="Reset View" @click="handleModalAction('reset')"><svg viewBox="0 0 24 24" v-html="ICON_PATHS['rotate-ccw']"></svg></button>
          <button class="mermaid-btn" data-action="fit" title="Fit to Screen" @click="handleModalAction('fit')"><svg viewBox="0 0 24 24" v-html="ICON_PATHS['maximize']"></svg></button>
          <button class="mermaid-btn close-btn" data-action="close" title="Close" @click="closeFullscreen"><svg viewBox="0 0 24 24" v-html="ICON_PATHS['x']"></svg></button>
        </div>
        <div class="mermaid-output fullscreen-output" ref="fullscreenOutputRef" v-html="fullscreenMermaidHTML"></div>
      </div>
    </dialog>
  </Teleport>
</template>
`
content = content.replace('</template>', dialogTemplate)

fs.writeFileSync(path, content)
