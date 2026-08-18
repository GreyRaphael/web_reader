<script setup lang="ts">
import {
  computed,
  defineAsyncComponent,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  watch,
} from 'vue'
import { getFileMeta, getSettings, getTextFile, getWorkspace, logout } from '@/api/client'
import { iconSvg } from '@/utils/icons'
import {
  storedBoolean,
  storedNumber,
  setStoredNumber,
  storedUnboundedNumber,
} from '@/utils/storage'
import type { FsItem, TabItem, TextResponse } from '@/api/types'
import DocumentTabs from '@/components/DocumentTabs.vue'
import FileTree from '@/components/FileTree.vue'
import OutlinePanel from '@/components/OutlinePanel.vue'
import PreviewPane from '@/components/PreviewPane.vue'
import SettingsModal from '@/components/SettingsModal.vue'
import ThemeControl from '@/components/ThemeControl.vue'
import { useTheme } from '@/composables/useTheme'

const TerminalPanel = defineAsyncComponent(() => import('@/components/TerminalPanel.vue'))
import type { MarkdownHeading } from '@/markdown/render'
import { decodeFragment } from '@/utils/path'
import { getPreviewMode, isTextPreview } from '@/utils/preview'

const props = defineProps<{ username: string }>()
const emit = defineEmits<{ signedOut: [] }>()
const { resolved } = useTheme()

const LEFT_VISIBLE_KEY = 'web-reader-left-visible'
const RIGHT_VISIBLE_KEY = 'web-reader-right-visible'
const LEFT_WIDTH_KEY = 'web-reader-left-width'
const RIGHT_WIDTH_KEY = 'web-reader-right-width'
const MIN_PANEL_WIDTH = 210
const MAX_PANEL_WIDTH = 480
const MIN_PREVIEW_WIDTH = 360

function storedWidth(key: string, fallback: number): number {
  return storedNumber(key, fallback, MIN_PANEL_WIDTH, MAX_PANEL_WIDTH)
}

const previewPane = ref<InstanceType<typeof PreviewPane> | null>(null)
const leftDrawer = ref<HTMLElement | null>(null)
const rightDrawer = ref<HTMLElement | null>(null)
const leftToggle = ref<HTMLButtonElement | null>(null)
const rightToggle = ref<HTMLButtonElement | null>(null)
const selectedItem = ref<FsItem | null>(null)
const textContent = ref<TextResponse | null>(null)
const loadingPreview = ref(false)
const previewError = ref('')
const toolbarMessage = ref('')
const headings = ref<MarkdownHeading[]>([])
const activeHeading = ref('')

const leftVisible = ref(storedBoolean(LEFT_VISIBLE_KEY, true))
const rightVisible = ref(storedBoolean(RIGHT_VISIBLE_KEY, true))
const mobileLeftOpen = ref(false)
const mobileRightOpen = ref(false)
const mobileViewport = ref(false)
const leftWidth = ref(storedWidth(LEFT_WIDTH_KEY, 286))
const rightWidth = ref(storedWidth(RIGHT_WIDTH_KEY, 264))
const fontSizeOffset = ref(storedUnboundedNumber('web-reader-font-offset', 0))
function changeFontSize(delta: number) {
  if (delta === 0) fontSizeOffset.value = 0
  else fontSizeOffset.value += delta
  setStoredNumber('web-reader-font-offset', fontSizeOffset.value)
}
const signingOut = ref(false)
const userMenuOpen = ref(false)
const showSettingsModal = ref(false)
const terminalOpen = ref(false)
const terminalEnabled = ref(true)
const terminalDialogRef = ref<HTMLDialogElement | null>(null)
const fileTreeKey = ref(0)
const workspaceRoot = ref('')

const OPEN_TABS_KEY = 'web-reader-open-tabs'
const RECENT_FILES_KEY = 'web-reader-recent-files'

function loadStoredTabs(): TabItem[] {
  try {
    const raw = window.localStorage.getItem(OPEN_TABS_KEY)
    if (raw) {
      const parsed = JSON.parse(raw)
      if (Array.isArray(parsed)) {
        return parsed.filter((t) => t && typeof t.path === 'string')
      }
    }
  } catch {
    // Ignore storage errors
  }
  return []
}

function saveStoredTabs(tabs: TabItem[]): void {
  try {
    window.localStorage.setItem(
      OPEN_TABS_KEY,
      JSON.stringify(
        tabs.map((t) => ({
          path: t.path,
          name: t.name,
          previewKind: t.previewKind,
          pinned: t.pinned,
        })),
      ),
    )
  } catch {
    // Ignore storage errors
  }
}

function loadRecentFiles(): string[] {
  try {
    const raw = window.localStorage.getItem(RECENT_FILES_KEY)
    if (raw) {
      const parsed = JSON.parse(raw)
      if (Array.isArray(parsed)) return parsed.filter((p) => typeof p === 'string')
    }
  } catch {
    // Ignore storage errors
  }
  return []
}

const openTabs = ref<TabItem[]>(loadStoredTabs())
const recentFiles = ref<string[]>(loadRecentFiles())

function addRecentFile(path: string): void {
  if (!path) return
  const current = loadRecentFiles().filter((p) => p !== path)
  current.unshift(path)
  const trimmed = current.slice(0, 20)
  recentFiles.value = trimmed
  try {
    window.localStorage.setItem(RECENT_FILES_KEY, JSON.stringify(trimmed))
  } catch {
    // Ignore storage errors
  }
}

function handleTabSelect(path: string): void {
  void openItem(path)
}

function handleTabClose(path: string): void {
  const index = openTabs.value.findIndex((t) => t.path === path)
  if (index === -1) return
  openTabs.value.splice(index, 1)
  saveStoredTabs(openTabs.value)

  if (selectedItem.value?.path === path) {
    if (openTabs.value.length > 0) {
      const nextIndex = Math.min(index, openTabs.value.length - 1)
      const nextTab = openTabs.value[nextIndex]
      if (nextTab) void openItem(nextTab.path)
    } else {
      selectedItem.value = null
      textContent.value = null
      headings.value = []
      activeHeading.value = ''
      updateLocation('', 'replace')
    }
  }
}

function handleTabCloseOthers(path: string): void {
  openTabs.value = openTabs.value.filter((t) => t.pinned || t.path === path)
  saveStoredTabs(openTabs.value)
  if (selectedItem.value?.path !== path) {
    void openItem(path)
  }
}

function handleTabCloseRight(path: string): void {
  const index = openTabs.value.findIndex((t) => t.path === path)
  if (index === -1) return
  openTabs.value = openTabs.value.filter((t, i) => i <= index || t.pinned)
  saveStoredTabs(openTabs.value)
  const currentStillOpen = openTabs.value.some((t) => t.path === selectedItem.value?.path)
  if (!currentStillOpen) {
    void openItem(path)
  }
}

function handleTabCloseAll(): void {
  const pinnedOnly = openTabs.value.filter((t) => t.pinned)
  openTabs.value = pinnedOnly
  saveStoredTabs(openTabs.value)
  if (pinnedOnly.length > 0) {
    const first = pinnedOnly[0]
    if (first) void openItem(first.path)
  } else {
    selectedItem.value = null
    textContent.value = null
    headings.value = []
    activeHeading.value = ''
    updateLocation('', 'replace')
  }
}

function handleTabTogglePin(path: string): void {
  const tab = openTabs.value.find((t) => t.path === path)
  if (!tab) return
  tab.pinned = !tab.pinned
  openTabs.value.sort((a, b) => {
    if (Boolean(a.pinned) === Boolean(b.pinned)) return 0
    return a.pinned ? -1 : 1
  })
  saveStoredTabs(openTabs.value)
}

function handleOpenRecent(path: string): void {
  void openItem(path)
}

function handleItemDeleted(item: FsItem): void {
  if (item.kind === 'directory') {
    const toClose = openTabs.value.filter(
      (t) => t.path === item.path || t.path.startsWith(`${item.path}/`),
    )
    for (const t of toClose) {
      handleTabClose(t.path)
    }
  } else {
    handleTabClose(item.path)
  }
}

function openSettings() {
  userMenuOpen.value = false
  showSettingsModal.value = true
}

function handleWorkspaceUpdated() {
  selectedItem.value = null
  textContent.value = null
  terminalOpen.value = false
  openTabs.value = []
  saveStoredTabs([])
  fileTreeKey.value++
  void getWorkspace()
    .then((res) => {
      workspaceRoot.value = res.workspace
    })
    .catch(() => {})
}

let previewRun = 0
let previewController: AbortController | null = null
let resizing: 'left' | 'right' | null = null
let resizeStartX = 0
let resizeStartWidth = 0
let drawerTrigger: HTMLElement | null = null
let lastPreview: { target: FsItem | string; hash: string } | null = null

const workspaceStyle = computed(() => ({
  '--left-column': leftVisible.value ? `${leftWidth.value}px` : '0px',
  '--left-grip': leftVisible.value ? '5px' : '0px',
  '--right-column': rightVisible.value ? `${rightWidth.value}px` : '0px',
  '--right-grip': rightVisible.value ? '5px' : '0px',
  '--markdown-font-size': `calc(clamp(14px, 1.25vw, 16px) + ${fontSizeOffset.value}px)`,
}))

function isMobile(): boolean {
  return mobileViewport.value
}

function eventTrigger(event?: MouseEvent): HTMLElement | null {
  return event?.currentTarget instanceof HTMLElement ? event.currentTarget : null
}

function toggleLeft(event?: MouseEvent): void {
  if (isMobile()) {
    drawerTrigger = eventTrigger(event) ?? leftToggle.value
    mobileLeftOpen.value = !mobileLeftOpen.value
    mobileRightOpen.value = false
    terminalOpen.value = false
  } else {
    leftVisible.value = !leftVisible.value
  }
}

function toggleRight(event?: MouseEvent): void {
  if (isMobile()) {
    drawerTrigger = eventTrigger(event) ?? rightToggle.value
    mobileRightOpen.value = !mobileRightOpen.value
    mobileLeftOpen.value = false
    terminalOpen.value = false
  } else {
    rightVisible.value = !rightVisible.value
  }
}

function closeDrawers(restoreFocus = true): void {
  const borderOpen = mobileLeftOpen.value || mobileRightOpen.value
  mobileLeftOpen.value = false
  mobileRightOpen.value = false
  if (!restoreFocus) drawerTrigger = null
  if (borderOpen && restoreFocus) {
    const trigger = drawerTrigger
    drawerTrigger = null
    void nextTick(() => trigger?.focus())
  }
}

function updateLocation(path: string, mode: 'push' | 'replace' | 'none', hash = ''): void {
  if (mode === 'none') return
  const url = new URL(window.location.href)
  if (path) {
    url.searchParams.set('path', path)
  } else {
    url.searchParams.delete('path')
  }
  url.hash = hash ? encodeURIComponent(hash) : ''
  window.history[mode === 'push' ? 'pushState' : 'replaceState']({}, '', url)
}

async function openItem(
  target: FsItem | string,
  hash = '',
  historyMode: 'push' | 'replace' | 'none' = 'push',
): Promise<void> {
  const run = ++previewRun
  lastPreview = { target, hash }
  previewController?.abort()
  previewController = new AbortController()
  loadingPreview.value = true
  previewError.value = ''
  textContent.value = null
  headings.value = []
  activeHeading.value = ''
  if (typeof target === 'string') selectedItem.value = null
  closeDrawers()

  try {
    const item =
      typeof target === 'string'
        ? (await getFileMeta(target, previewController.signal)).item
        : target
    if (run !== previewRun) return
    if (item.kind !== 'file') throw new Error('目录不能在预览区打开')

    selectedItem.value = item
    const existingIndex = openTabs.value.findIndex((t) => t.path === item.path)
    if (existingIndex === -1) {
      openTabs.value.push({
        path: item.path,
        name: item.name,
        previewKind: item.previewKind,
        pinned: false,
      })
    } else {
      const tab = openTabs.value[existingIndex]
      if (tab) {
        tab.name = item.name
        tab.previewKind = item.previewKind
      }
    }
    saveStoredTabs(openTabs.value)
    addRecentFile(item.path)

    if (getPreviewMode(item) === 'markdown') {
      rightVisible.value = true
    } else {
      rightVisible.value = false
    }
    updateLocation(item.path, historyMode, hash)
    if (isTextPreview(item.previewKind)) {
      const text = await getTextFile(item.path, previewController.signal)
      if (run !== previewRun) return
      textContent.value = text
    }
    loadingPreview.value = false

    if (hash) {
      await nextTick()
      window.requestAnimationFrame(() => previewPane.value?.scrollToHeading(hash))
    }
  } catch (error) {
    if (error instanceof Error && error.name === 'AbortError') return
    if (run !== previewRun) return
    previewError.value = error instanceof Error ? error.message : '文件读取失败'
    loadingPreview.value = false
  }
}

function retryPreview(): void {
  if (lastPreview) void openItem(lastPreview.target, lastPreview.hash, 'none')
}

async function handleFileSaved(path: string): Promise<void> {
  try {
    const res = await getTextFile(path)
    if (selectedItem.value && selectedItem.value.path === path) {
      textContent.value = res
    }
  } catch {
    retryPreview()
  }
}

function handleTreeOpen(item: FsItem): void {
  void openItem(item)
}

function handleInternalOpen(path: string, hash: string): void {
  void openItem(path, hash)
}

function selectHeading(id: string): void {
  previewPane.value?.scrollToHeading(id)
  if (selectedItem.value) updateLocation(selectedItem.value.path, 'push', id)
  closeDrawers()
}

function startResize(side: 'left' | 'right', event: PointerEvent): void {
  if (isMobile()) return
  resizing = side
  resizeStartX = event.clientX
  resizeStartWidth = side === 'left' ? leftWidth.value : rightWidth.value
  document.body.classList.add('is-resizing')
  window.addEventListener('pointermove', resizeMove)
  window.addEventListener('pointerup', stopResize, { once: true })
  window.addEventListener('pointercancel', stopResize, { once: true })
  event.preventDefault()
}

function clampPanelWidth(value: number): number {
  return Math.min(MAX_PANEL_WIDTH, Math.max(MIN_PANEL_WIDTH, value))
}

function resizeMove(event: PointerEvent): void {
  if (!resizing) return
  const delta = event.clientX - resizeStartX
  const proposed = resizing === 'left' ? resizeStartWidth + delta : resizeStartWidth - delta
  const width = clampPanelWidth(proposed)
  if (resizing === 'left') leftWidth.value = width
  else rightWidth.value = width
  constrainPanelWidths()
}

function stopResize(): void {
  resizing = null
  document.body.classList.remove('is-resizing')
  window.removeEventListener('pointermove', resizeMove)
  window.removeEventListener('pointercancel', stopResize)
}

function resizeWithKeyboard(side: 'left' | 'right', event: KeyboardEvent): void {
  const current = side === 'left' ? leftWidth.value : rightWidth.value
  let next = current
  if (event.key === 'Home') next = MIN_PANEL_WIDTH
  else if (event.key === 'End') next = MAX_PANEL_WIDTH
  else if (event.key === 'ArrowLeft') next += side === 'left' ? -16 : 16
  else if (event.key === 'ArrowRight') next += side === 'left' ? 16 : -16
  else return
  event.preventDefault()
  if (side === 'left') leftWidth.value = clampPanelWidth(next)
  else rightWidth.value = clampPanelWidth(next)
  constrainPanelWidths()
}

function constrainPanelWidths(): void {
  if (isMobile()) return
  const visibleCount = Number(leftVisible.value) + Number(rightVisible.value)
  if (visibleCount === 0) return
  const available = Math.max(
    MIN_PANEL_WIDTH * visibleCount,
    window.innerWidth - MIN_PREVIEW_WIDTH - visibleCount * 5,
  )
  let overflow =
    (leftVisible.value ? leftWidth.value : 0) +
    (rightVisible.value ? rightWidth.value : 0) -
    available
  if (overflow <= 0) return
  if (rightVisible.value) {
    const reduction = Math.min(overflow, rightWidth.value - MIN_PANEL_WIDTH)
    rightWidth.value -= reduction
    overflow -= reduction
  }
  if (overflow > 0 && leftVisible.value) {
    leftWidth.value = Math.max(MIN_PANEL_WIDTH, leftWidth.value - overflow)
  }
}

function handleViewportChange(): void {
  mobileViewport.value = window.matchMedia('(max-width: 840px)').matches
  if (!mobileViewport.value) closeDrawers(false)
  constrainPanelWidths()
}

async function signOut(): Promise<void> {
  if (signingOut.value) return
  signingOut.value = true
  toolbarMessage.value = ''
  try {
    await logout()
    emit('signedOut')
  } catch (error) {
    toolbarMessage.value = error instanceof Error ? error.message : '退出失败'
  } finally {
    signingOut.value = false
  }
}

function handleKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape' && (mobileLeftOpen.value || mobileRightOpen.value)) {
    event.preventDefault()
    closeDrawers()
    return
  }
  if (event.key === 'Escape' && terminalOpen.value && mobileViewport.value) {
    event.preventDefault()
    terminalOpen.value = false
    return
  }
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'w') {
    if (selectedItem.value) {
      event.preventDefault()
      handleTabClose(selectedItem.value.path)
      return
    }
  }
  if (
    (event.altKey || (event.ctrlKey && event.shiftKey)) &&
    (event.key === 'ArrowRight' || event.key === 'PageDown')
  ) {
    if (openTabs.value.length > 1 && selectedItem.value) {
      const idx = openTabs.value.findIndex((t) => t.path === selectedItem.value?.path)
      if (idx !== -1) {
        event.preventDefault()
        const nextIdx = (idx + 1) % openTabs.value.length
        const nextTab = openTabs.value[nextIdx]
        if (nextTab) void openItem(nextTab.path)
        return
      }
    }
  }
  if (
    (event.altKey || (event.ctrlKey && event.shiftKey)) &&
    (event.key === 'ArrowLeft' || event.key === 'PageUp')
  ) {
    if (openTabs.value.length > 1 && selectedItem.value) {
      const idx = openTabs.value.findIndex((t) => t.path === selectedItem.value?.path)
      if (idx !== -1) {
        event.preventDefault()
        const prevIdx = (idx - 1 + openTabs.value.length) % openTabs.value.length
        const prevTab = openTabs.value[prevIdx]
        if (prevTab) void openItem(prevTab.path)
        return
      }
    }
  }
  if (event.key !== 'Tab') return
  const drawer = mobileLeftOpen.value
    ? leftDrawer.value
    : mobileRightOpen.value
      ? rightDrawer.value
      : null
  if (!drawer) return
  const focusable = Array.from(
    drawer.querySelectorAll<HTMLElement>(
      'button:not([disabled]), a[href], [tabindex]:not([tabindex="-1"])',
    ),
  )
  if (focusable.length === 0) return
  const first = focusable[0]
  const last = focusable.at(-1)
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last?.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first?.focus()
  }
}

function handlePopState(): void {
  const path = new URL(window.location.href).searchParams.get('path')
  if (path) void openItem(path, decodeFragment(window.location.hash.slice(1)), 'none')
}

watch([leftVisible, rightVisible, leftWidth, rightWidth], () => {
  try {
    window.localStorage.setItem(LEFT_VISIBLE_KEY, String(leftVisible.value))
    window.localStorage.setItem(RIGHT_VISIBLE_KEY, String(rightVisible.value))
    window.localStorage.setItem(LEFT_WIDTH_KEY, String(leftWidth.value))
    window.localStorage.setItem(RIGHT_WIDTH_KEY, String(rightWidth.value))
  } catch {
    // Layout preferences remain active for the current page when storage is unavailable.
  }
})

watch([mobileLeftOpen, mobileRightOpen], async ([leftOpen, rightOpen]) => {
  if (!leftOpen && !rightOpen) return
  await nextTick()
  const drawer = leftOpen ? leftDrawer.value : rightDrawer.value
  drawer
    ?.querySelector<HTMLElement>('button:not([disabled]), a[href], [tabindex]:not([tabindex="-1"])')
    ?.focus()
})

watch([terminalOpen, mobileViewport], async ([open, mobile]) => {
  if (!open || !mobile) return
  await nextTick()
  const dialog = terminalDialogRef.value
  if (dialog && !dialog.open) dialog.showModal()
})
function handleGlobalClick(e: MouseEvent) {
  if (userMenuOpen.value && !(e.target as Element).closest('.user-menu-container')) {
    userMenuOpen.value = false
  }
}
onMounted(() => {
  document.addEventListener('click', handleGlobalClick)
  handleViewportChange()
  constrainPanelWidths()
  window.addEventListener('keydown', handleKeydown)
  window.addEventListener('popstate', handlePopState)
  window.addEventListener('resize', handleViewportChange)
  void getWorkspace()
    .then((res) => {
      workspaceRoot.value = res.workspace
    })
    .catch(() => {})
  void getSettings()
    .then((res) => (terminalEnabled.value = res.enableTerminal))
    .catch(() => {})
  const url = new URL(window.location.href)
  const path = url.searchParams.get('path')
  if (path) {
    void openItem(path, decodeFragment(url.hash.slice(1)), 'none')
  } else if (openTabs.value.length > 0) {
    const first = openTabs.value[0]
    if (first) void openItem(first.path, '', 'none')
  }
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleGlobalClick)
  previewController?.abort()
  stopResize()
  window.removeEventListener('keydown', handleKeydown)
  window.removeEventListener('popstate', handlePopState)
  window.removeEventListener('resize', handleViewportChange)
})
</script>

<template>
  <div class="reader-view">
    <header class="app-toolbar">
      <div class="toolbar-section toolbar-start">
        <button
          id="left-panel-toggle"
          ref="leftToggle"
          class="icon-button"
          type="button"
          :aria-expanded="mobileViewport ? mobileLeftOpen : leftVisible"
          aria-controls="left-panel"
          aria-label="切换文件栏"
          title="切换文件栏"
          @click="toggleLeft"
          v-html="iconSvg('menu', 18)"
        ></button>
        <div class="toolbar-brand" title="Web Reader">
          <span class="mini-brand">W</span>
          <span>Web Reader</span>
        </div>
      </div>

      <p class="toolbar-path" :title="selectedItem?.path">
        {{ selectedItem?.path || '选择一个文件开始阅读' }}
      </p>

      <div class="toolbar-section toolbar-end">
        <ThemeControl />
        <div class="font-controls">
          <button
            class="font-btn"
            type="button"
            @click="changeFontSize(-1)"
            title="缩小字体"
            aria-label="缩小字体"
          >
            A-
          </button>
          <button
            class="font-btn"
            type="button"
            @click="changeFontSize(0)"
            title="重置字体大小"
            aria-label="重置字体大小"
          >
            Aa
          </button>
          <button
            class="font-btn"
            type="button"
            @click="changeFontSize(1)"
            title="放大字体"
            aria-label="放大字体"
          >
            A+
          </button>
        </div>
        <button
          v-if="terminalEnabled"
          class="icon-button compact terminal-toggle"
          type="button"
          :class="{ active: terminalOpen }"
          :aria-expanded="terminalOpen"
          aria-label="终端"
          title="终端"
          v-html="iconSvg('terminal', 18)"
          @click="terminalOpen = !terminalOpen"
        ></button>
        <div class="user-menu-container">
          <button
            class="user-avatar"
            type="button"
            @click="userMenuOpen = !userMenuOpen"
            :title="props.username"
            v-html="iconSvg('user', 16)"
          ></button>
          <Transition name="dropdown">
            <div v-if="userMenuOpen" class="user-dropdown">
              <button class="user-dropdown-btn" type="button" @click="openSettings">
                <span>⚙️</span>
                设置 (Settings)
              </button>
              <button
                class="user-dropdown-btn danger"
                type="button"
                :disabled="signingOut"
                @click="signOut"
              >
                <span v-html="iconSvg('log-out', 14)"></span>
                {{ signingOut ? '退出中…' : '退出登录' }}
              </button>
            </div>
          </Transition>
        </div>
      </div>
    </header>

    <p v-if="toolbarMessage" class="toolbar-message" role="alert">{{ toolbarMessage }}</p>

    <main class="reader-workspace" :style="workspaceStyle">
      <aside
        id="left-panel"
        ref="leftDrawer"
        class="side-panel left-panel"
        :class="{ 'desktop-collapsed': !leftVisible, 'mobile-open': mobileLeftOpen }"
        :role="mobileViewport ? 'dialog' : 'complementary'"
        :aria-modal="mobileViewport ? 'true' : undefined"
        :aria-label="mobileViewport ? '工作区文件' : undefined"
        :inert="mobileViewport ? !mobileLeftOpen || undefined : !leftVisible || undefined"
      >
        <FileTree
          :key="fileTreeKey"
          :selected-path="selectedItem?.path || ''"
          @open="handleTreeOpen"
          @close="closeDrawers()"
          @open-settings="openSettings"
          @deleted="handleItemDeleted"
        />
      </aside>

      <div
        class="panel-resizer left-resizer"
        role="separator"
        aria-label="调整文件栏宽度"
        aria-orientation="vertical"
        :aria-valuemin="MIN_PANEL_WIDTH"
        :aria-valuemax="MAX_PANEL_WIDTH"
        :aria-valuenow="leftWidth"
        :tabindex="leftVisible ? 0 : -1"
        @keydown="resizeWithKeyboard('left', $event)"
        @pointerdown="startResize('left', $event)"
      ></div>

      <div class="workspace-main">
        <DocumentTabs
          v-if="openTabs.length > 0"
          :tabs="openTabs"
          :active-path="selectedItem?.path || ''"
          :workspace-root="workspaceRoot"
          :recent-files="recentFiles"
          @select="handleTabSelect"
          @close="handleTabClose"
          @close-others="handleTabCloseOthers"
          @close-right="handleTabCloseRight"
          @close-all="handleTabCloseAll"
          @toggle-pin="handleTabTogglePin"
          @open-recent="handleOpenRecent"
        />
        <PreviewPane
          ref="previewPane"
          :item="selectedItem"
          :text="textContent"
          :loading="loadingPreview"
          :error="previewError"
          :theme="resolved"
          :outline-open="mobileViewport ? mobileRightOpen : rightVisible"
          :inert="mobileLeftOpen || mobileRightOpen || undefined"
          @headings="headings = $event"
          @active-heading="activeHeading = $event"
          @open-path="handleInternalOpen"
          @retry="retryPreview"
          @saved="handleFileSaved"
          @toggle-outline="toggleRight()"
        />
      </div>

      <div
        class="panel-resizer right-resizer"
        role="separator"
        aria-label="调整大纲栏宽度"
        aria-orientation="vertical"
        :aria-valuemin="MIN_PANEL_WIDTH"
        :aria-valuemax="MAX_PANEL_WIDTH"
        :aria-valuenow="rightWidth"
        :tabindex="rightVisible ? 0 : -1"
        @keydown="resizeWithKeyboard('right', $event)"
        @pointerdown="startResize('right', $event)"
      ></div>

      <aside
        id="right-panel"
        ref="rightDrawer"
        class="side-panel right-panel"
        :class="{ 'desktop-collapsed': !rightVisible, 'mobile-open': mobileRightOpen }"
        :role="mobileViewport ? 'dialog' : 'complementary'"
        :aria-modal="mobileViewport ? 'true' : undefined"
        :aria-label="mobileViewport ? '文章大纲' : undefined"
        :inert="mobileViewport ? !mobileRightOpen || undefined : !rightVisible || undefined"
      >
        <OutlinePanel
          :headings="headings"
          :active-id="activeHeading"
          @select="selectHeading"
          @close="closeDrawers()"
        />
      </aside>

      <button
        v-if="mobileLeftOpen || mobileRightOpen"
        class="drawer-backdrop"
        type="button"
        aria-label="关闭侧栏"
        @click="closeDrawers()"
      ></button>
    </main>

    <SettingsModal
      v-if="showSettingsModal"
      :username="props.username"
      @close="showSettingsModal = false"
      @updated="handleWorkspaceUpdated"
      @settings-changed="terminalEnabled = $event"
    />

    <!-- Terminal: mobile full-screen dialog, desktop bottom panel -->
    <Teleport to="body">
      <dialog
        ref="terminalDialogRef"
        v-if="terminalOpen && mobileViewport"
        class="terminal-dialog"
        @cancel.prevent="terminalOpen = false"
      >
        <TerminalPanel :theme="resolved" @close="terminalOpen = false" />
      </dialog>
    </Teleport>
    <div
      v-if="terminalOpen && !mobileViewport"
      class="terminal-dock"
      :inert="mobileLeftOpen || mobileRightOpen || undefined"
    >
      <TerminalPanel :theme="resolved" @close="terminalOpen = false" />
    </div>
  </div>
</template>
