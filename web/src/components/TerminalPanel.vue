<script setup lang="ts">
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { ResolvedTheme } from '@/composables/useTheme'
import TerminalKeyBar from './TerminalKeyBar.vue'

const props = defineProps<{ theme: ResolvedTheme }>()
const emit = defineEmits<{ close: [] }>()

const containerRef = ref<HTMLElement | null>(null)
let term: Terminal | null = null
let fitAddon: FitAddon | null = null
let ws: WebSocket | null = null
let resizeObserver: ResizeObserver | null = null

function buildTheme(theme: ResolvedTheme) {
  if (theme === 'night') {
    return {
      background: '#0d110f',
      foreground: '#e4e9e4',
      cursor: '#e4e9e4',
      selectionBackground: '#28312b',
    }
  }
  return {
    background: '#fbfbf8',
    foreground: '#202521',
    cursor: '#202521',
    selectionBackground: '#dceadf',
  }
}

function sendResize(cols: number, rows: number) {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({ type: 'resize', cols, rows }))
  }
}

function fitTerminal() {
  if (!fitAddon || !term) return
  fitAddon.fit()
  sendResize(term.cols, term.rows)
}

function handleViewportResize() {
  fitTerminal()
}

function connect() {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  ws = new WebSocket(`${proto}//${window.location.host}/api/terminal`)
  ws.binaryType = 'arraybuffer'
  ws.onmessage = (event) => {
    if (term && event.data instanceof ArrayBuffer) {
      term.write(new Uint8Array(event.data))
    } else if (term && typeof event.data === 'string') {
      term.write(event.data)
    }
  }
  ws.onclose = () => {
    term?.write('\r\n\x1b[33m[连接已断开]\x1b[0m\r\n')
  }
  ws.onerror = () => {
    term?.write('\r\n\x1b[31m[连接错误]\x1b[0m\r\n')
  }
}

function sendData(data: string) {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(data)
  }
}

onMounted(() => {
  if (!containerRef.value) return
  term = new Terminal({
    fontFamily: "'SFMono-Regular', Consolas, 'Liberation Mono', 'Menlo', monospace",
    fontSize: 12,
    cursorBlink: true,
    scrollback: 1000,
    theme: buildTheme(props.theme),
  })
  fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.open(containerRef.value)
  term.onData((data) => sendData(data))
  connect()
  requestAnimationFrame(() => fitTerminal())

  resizeObserver = new ResizeObserver(() => fitTerminal())
  resizeObserver.observe(containerRef.value)
  if (window.visualViewport) {
    window.visualViewport.addEventListener('resize', handleViewportResize)
  }
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  if (window.visualViewport) {
    window.visualViewport.removeEventListener('resize', handleViewportResize)
  }
  ws?.close()
  term?.dispose()
  term = null
  fitAddon = null
})

watch(
  () => props.theme,
  (newTheme) => {
    if (term) term.options.theme = buildTheme(newTheme)
  },
)
</script>

<template>
  <div class="terminal-panel">
    <div class="terminal-header">
      <span class="terminal-title">终端</span>
      <button class="terminal-close" type="button" aria-label="关闭终端" @click="emit('close')">
        ×
      </button>
    </div>
    <TerminalKeyBar @key="sendData" />
    <div ref="containerRef" class="terminal-container"></div>
  </div>
</template>

<style scoped>
.terminal-panel {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  background: #0d110f;
  overflow: hidden;
}

.terminal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 12px;
  border-bottom: 1px solid var(--border);
  background: var(--surface-raised);
}

.terminal-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.terminal-close {
  border: none;
  background: transparent;
  color: var(--text-muted);
  font-size: 20px;
  line-height: 1;
  cursor: pointer;
}

.terminal-close:hover {
  color: var(--text);
}

.terminal-container {
  flex: 1;
  min-height: 0;
  padding: 4px 8px;
  overflow: hidden;
}

.terminal-container :deep(.xterm) {
  padding: 4px;
}

.terminal-container :deep(.xterm-viewport) {
  overflow-y: auto;
}
</style>
