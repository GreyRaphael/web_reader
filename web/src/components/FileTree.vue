<script setup lang="ts">
import { onBeforeUnmount, onMounted, reactive, ref, watch, provide } from 'vue'
import {
  createDir,
  createFile,
  deleteFile,
  getWorkspace,
  listDirectory,
  moveFile,
  rawFileUrl,
  renameFile,
  uploadFile,
  zipUrl,
} from '@/api/client'
import type { FsItem } from '@/api/types'
import { iconSvg, fileIconName } from '@/utils/icons'
import { resolveAbsolutePath } from '@/utils/path'
import { sortFileItems } from '@/utils/sort'
import type { ContextMenuItem } from './ContextMenu.vue'
import ContextMenu from './ContextMenu.vue'
import TreeChildren from './TreeChildren.vue'

const props = defineProps<{ selectedPath: string }>()
const emit = defineEmits<{ open: [item: FsItem]; close: []; openSettings: [] }>()

const workspaceRoot = ref('')
const items = ref<FsItem[]>([])
const loading = ref(true)
const errorMessage = ref('')
const toolMessage = ref('')
const currentDir = ref('')
const workingDir = ref('')
let suppressNavigate = false
let controller: AbortController | null = null
let loadRun = 0

const contextMenu = ref<{ x: number; y: number; items: ContextMenuItem[] } | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)
const dragOverDir = ref<string | null>(null)

const breadcrumb = ref<{ name: string; path: string }[]>([{ name: '~', path: '' }])

const expandedDirs = reactive(new Set<string>())
const childCache = reactive(new Map<string, FsItem[]>())
const childLoading = reactive(new Set<string>())
const childError = reactive(new Map<string, string>())

function buildBreadcrumb(dir: string): void {
  if (!dir) {
    breadcrumb.value = [{ name: '~', path: '' }]
    return
  }
  const parts = dir.split('/')
  breadcrumb.value = [{ name: '~', path: '' }]
  let accumulated = ''
  for (const part of parts) {
    accumulated = accumulated ? `${accumulated}/${part}` : part
    breadcrumb.value.push({ name: part, path: accumulated })
  }
}

async function loadDir(dir: string): Promise<void> {
  const run = ++loadRun
  controller?.abort()
  controller = new AbortController()
  loading.value = true
  errorMessage.value = ''
  currentDir.value = dir
  if (dir === currentDir.value || !workingDir.value) workingDir.value = dir
  buildBreadcrumb(dir)
  try {
    const response = await listDirectory(dir, controller.signal)
    if (run !== loadRun) return
    items.value = sortFileItems(response.items)
  } catch (error) {
    if (error instanceof Error && error.name === 'AbortError') return
    if (run !== loadRun) return
    errorMessage.value = error instanceof Error ? error.message : '文件列表加载失败'
  } finally {
    if (run === loadRun) loading.value = false
  }
}

function navigateTo(dir: string): void {
  workingDir.value = dir
  void loadDir(dir)
}

function refreshDir(): void {
  void loadDir(currentDir.value)
  for (const dir of expandedDirs) {
    void loadChildren(dir)
  }
}

async function loadChildren(dir: string): Promise<void> {
  if (childLoading.has(dir)) return
  childLoading.add(dir)
  childError.delete(dir)
  try {
    const response = await listDirectory(dir)
    childCache.set(dir, sortFileItems(response.items))
  } catch (error) {
    childError.set(dir, error instanceof Error ? error.message : '加载失败')
  } finally {
    childLoading.delete(dir)
  }
}

provide('expandedDirs', expandedDirs)
provide('childCache', childCache)
provide('childLoading', childLoading)
provide('loadChildren', loadChildren)

function toggleExpand(dir: string): void {
  if (expandedDirs.has(dir)) {
    expandedDirs.delete(dir)
  } else {
    expandedDirs.add(dir)
    if (!childCache.has(dir)) void loadChildren(dir)
  }
}

function collapseAll(): void {
  expandedDirs.clear()
}

function isExpanded(dir: string): boolean {
  return expandedDirs.has(dir)
}

function getChildren(dir: string): FsItem[] {
  return childCache.get(dir) ?? []
}

function getChildError(dir: string): string {
  return childError.get(dir) ?? ''
}

function isChildLoading(dir: string): boolean {
  return childLoading.has(dir)
}

async function createWithPrompt(
  label: string,
  apiCall: (path: string) => Promise<unknown>,
  errorMsg: string,
  parentDir = '',
): Promise<void> {
  const name = prompt(label)
  if (!name) return
  const dir = parentDir || workingDir.value
  const fullPath = dir ? `${dir}/${name}` : name
  try {
    await apiCall(fullPath)
    toolMessage.value = ''
    refreshDir()
  } catch (error) {
    toolMessage.value = error instanceof Error ? error.message : errorMsg
  }
}

async function handleCreateFile(): Promise<void> {
  await createWithPrompt('文件名：', createFile, '创建文件失败')
}

async function handleCreateDir(): Promise<void> {
  await createWithPrompt('文件夹名：', createDir, '创建文件夹失败')
}

function handleUploadClick(): void {
  fileInput.value?.click()
}

const uploadQueue = ref<File[]>([])
const uploadingCount = ref(0)
const uploadTotal = ref(0)
const uploadSuccessCount = ref(0)
const uploadFailedFiles = ref<string[]>([])
const MAX_CONCURRENT_UPLOADS = 3
let toolMessageTimer: number | null = null

function showToolMessage(msg: string, autoClearMs = 0): void {
  if (toolMessageTimer) {
    clearTimeout(toolMessageTimer)
    toolMessageTimer = null
  }
  toolMessage.value = msg
  if (autoClearMs > 0 && msg) {
    toolMessageTimer = window.setTimeout(() => {
      toolMessage.value = ''
      toolMessageTimer = null
    }, autoClearMs)
  }
}

async function processUploadQueue(dir: string): Promise<void> {
  if (uploadQueue.value.length === 0) {
    if (uploadingCount.value === 0) {
      const totalSuccess = uploadSuccessCount.value
      const totalFailed = uploadFailedFiles.value.length
      if (totalFailed === 0) {
        showToolMessage(totalSuccess > 0 ? `成功上传 ${totalSuccess} 个文件` : '', 3000)
      } else if (totalSuccess === 0) {
        const failedSummary =
          uploadFailedFiles.value.slice(0, 2).join(', ') + (totalFailed > 2 ? ' 等' : '')
        showToolMessage(`上传失败 (${totalFailed} 个文件)：${failedSummary}`)
      } else {
        const failedSummary =
          uploadFailedFiles.value.slice(0, 2).join(', ') + (totalFailed > 2 ? ' 等' : '')
        showToolMessage(
          `部分上传成功（${totalSuccess} 成功，${totalFailed} 失败：${failedSummary}）`,
        )
      }
      uploadTotal.value = 0
      uploadSuccessCount.value = 0
      uploadFailedFiles.value = []
      refreshDir()
    }
    return
  }

  while (uploadingCount.value < MAX_CONCURRENT_UPLOADS && uploadQueue.value.length > 0) {
    const file = uploadQueue.value.shift()
    if (!file) continue

    uploadingCount.value++
    const fullPath = dir ? `${dir}/${file.name}` : file.name
    showToolMessage(
      `正在上传 (${uploadTotal.value - uploadQueue.value.length}/${uploadTotal.value})...`,
    )

    file
      .arrayBuffer()
      .then((buffer) => uploadFile(fullPath, buffer))
      .then(() => {
        uploadSuccessCount.value++
      })
      .catch((error) => {
        console.error(`上传失败 ${file.name}:`, error)
        uploadFailedFiles.value.push(file.name)
      })
      .finally(() => {
        uploadingCount.value--
        processUploadQueue(dir)
      })
  }
}

function handleUpload(event: Event): void {
  const input = event.target as HTMLInputElement
  if (!input.files || input.files.length === 0) return
  const files = Array.from(input.files)

  uploadQueue.value.push(...files)
  uploadTotal.value += files.length

  const dir = workingDir.value
  processUploadQueue(dir)
  input.value = ''
}

async function handleDelete(item: FsItem): Promise<void> {
  if (!confirm(`确定删除 ${item.path}？`)) return
  try {
    await deleteFile(item.path)
    expandedDirs.delete(item.path)
    childCache.delete(item.path)
    toolMessage.value = ''
    refreshDir()
  } catch (error) {
    toolMessage.value = error instanceof Error ? error.message : '删除失败'
  }
}

function handleChevron(item: FsItem): void {
  toggleExpand(item.path)
}

function handleRowClick(item: FsItem): void {
  if (item.kind === 'directory') {
    navigateTo(item.path)
  } else {
    const lastSlash = item.path.lastIndexOf('/')
    const dir = lastSlash >= 0 ? item.path.slice(0, lastSlash) : ''
    workingDir.value = dir
    if (dir !== currentDir.value) suppressNavigate = true
    emit('open', item)
  }
}

function handleChildClick(item: FsItem): void {
  handleRowClick(item)
}

function buildContextMenu(target: FsItem, event: MouseEvent): void {
  event.preventDefault()
  event.stopPropagation()
  const parentDir =
    target.kind === 'directory'
      ? target.path
      : target.path.lastIndexOf('/') >= 0
        ? target.path.slice(0, target.path.lastIndexOf('/'))
        : ''
  const items: ContextMenuItem[] = [
    {
      label: '新建文件',
      icon: 'file-text',
      action: () => createWithPrompt('文件名：', createFile, '创建文件失败', parentDir),
    },
    {
      label: '新建文件夹',
      icon: 'folder',
      action: () => createWithPrompt('文件夹名：', createDir, '创建文件夹失败', parentDir),
    },
    {
      label: '重命名',
      icon: 'pencil',
      action: async () => {
        const newName = prompt('新名称：', target.name)
        if (!newName) return
        try {
          await renameFile(target.path, newName)
          refreshDir()
        } catch (error) {
          toolMessage.value = error instanceof Error ? error.message : '重命名失败'
        }
      },
    },
    { label: '', action: () => {}, separator: true },
    {
      label: '复制绝对路径',
      icon: 'copy',
      action: async () => {
        try {
          let root = workspaceRoot.value
          if (!root) {
            const res = await getWorkspace()
            root = res.workspace
            workspaceRoot.value = root
          }
          const absPath = resolveAbsolutePath(root, target.path)
          await navigator.clipboard.writeText(absPath)
        } catch {
          // clipboard API unavailable
        }
      },
    },
    {
      label: '复制相对路径',
      icon: 'link',
      action: async () => {
        try {
          await navigator.clipboard.writeText(target.path)
        } catch {
          // clipboard API unavailable
        }
      },
    },
    {
      label: target.kind === 'directory' ? '下载 ZIP' : '下载',
      icon: 'download',
      action: () => {
        const url =
          target.kind === 'directory' ? zipUrl(target.path) : rawFileUrl(target.path, true)
        window.open(url, '_blank')
      },
    },
    { label: '', action: () => {}, separator: true },
    {
      label: '删除',
      icon: 'trash-2',
      danger: true,
      action: () => handleDelete(target),
    },
  ]
  contextMenu.value = { x: event.clientX, y: event.clientY, items }
}

function handleBreadcrumbContextMenu(
  crumb: { name: string; path: string },
  event: MouseEvent,
): void {
  event.preventDefault()
  event.stopPropagation()
  if (crumb.path === '') {
    const items: ContextMenuItem[] = [
      {
        label: '切换工作区路径...',
        icon: 'folder-open',
        action: () => emit('openSettings'),
      },
      {
        label: '复制工作区绝对路径',
        icon: 'copy',
        action: async () => {
          try {
            let root = workspaceRoot.value
            if (!root) {
              const res = await getWorkspace()
              root = res.workspace
              workspaceRoot.value = root
            }
            if (root) await navigator.clipboard.writeText(root)
          } catch {
            // clipboard unavailable
          }
        },
      },
      { label: '', action: () => {}, separator: true },
      {
        label: '新建文件',
        icon: 'file-text',
        action: () => createWithPrompt('文件名：', createFile, '创建文件失败', ''),
      },
      {
        label: '新建文件夹',
        icon: 'folder',
        action: () => createWithPrompt('文件夹名：', createDir, '创建文件夹失败', ''),
      },
      { label: '', action: () => {}, separator: true },
      {
        label: '刷新工作区',
        icon: 'refresh-cw',
        action: () => refreshDir(),
      },
      {
        label: '折叠所有文件夹',
        icon: 'collapse-all',
        action: () => collapseAll(),
      },
    ]
    contextMenu.value = { x: event.clientX, y: event.clientY, items }
  } else {
    const items: ContextMenuItem[] = [
      {
        label: '复制绝对路径',
        icon: 'copy',
        action: async () => {
          try {
            let root = workspaceRoot.value
            if (!root) {
              const res = await getWorkspace()
              root = res.workspace
              workspaceRoot.value = root
            }
            const absPath = resolveAbsolutePath(root, crumb.path)
            await navigator.clipboard.writeText(absPath)
          } catch {
            // clipboard unavailable
          }
        },
      },
      {
        label: '复制相对路径',
        icon: 'link',
        action: async () => {
          try {
            await navigator.clipboard.writeText(crumb.path)
          } catch {
            // clipboard unavailable
          }
        },
      },
      { label: '', action: () => {}, separator: true },
      {
        label: '新建文件',
        icon: 'file-text',
        action: () => createWithPrompt('文件名：', createFile, '创建文件失败', crumb.path),
      },
      {
        label: '新建文件夹',
        icon: 'folder',
        action: () => createWithPrompt('文件夹名：', createDir, '创建文件夹失败', crumb.path),
      },
      { label: '', action: () => {}, separator: true },
      {
        label: '进入此目录',
        icon: 'folder',
        action: () => navigateTo(crumb.path),
      },
    ]
    contextMenu.value = { x: event.clientX, y: event.clientY, items }
  }
}

function closeContextMenu(): void {
  contextMenu.value = null
}

function buildBlankContextMenu(event: MouseEvent): void {
  event.preventDefault()
  event.stopPropagation()
  const dir = currentDir.value
  const items: ContextMenuItem[] = [
    {
      label: '新建文件',
      icon: 'file-text',
      action: () => createWithPrompt('文件名：', createFile, '创建文件失败', dir),
    },
    {
      label: '新建文件夹',
      icon: 'folder',
      action: () => createWithPrompt('文件夹名：', createDir, '创建文件夹失败', dir),
    },
    {
      label: '上传文件...',
      icon: 'upload',
      action: () => handleUploadClick(),
    },
    { label: '', action: () => {}, separator: true },
  ]

  if (!dir) {
    items.push(
      {
        label: '切换工作区路径...',
        icon: 'folder-open',
        action: () => emit('openSettings'),
      },
      {
        label: '复制工作区绝对路径',
        icon: 'copy',
        action: async () => {
          try {
            let root = workspaceRoot.value
            if (!root) {
              const res = await getWorkspace()
              root = res.workspace
              workspaceRoot.value = root
            }
            if (root) await navigator.clipboard.writeText(root)
          } catch {
            // clipboard unavailable
          }
        },
      },
    )
  } else {
    items.push(
      {
        label: '复制绝对路径',
        icon: 'copy',
        action: async () => {
          try {
            let root = workspaceRoot.value
            if (!root) {
              const res = await getWorkspace()
              root = res.workspace
              workspaceRoot.value = root
            }
            const absPath = resolveAbsolutePath(root, dir)
            await navigator.clipboard.writeText(absPath)
          } catch {
            // clipboard unavailable
          }
        },
      },
      {
        label: '复制相对路径',
        icon: 'link',
        action: async () => {
          try {
            await navigator.clipboard.writeText(dir)
          } catch {
            // clipboard unavailable
          }
        },
      },
    )
  }

  items.push(
    { label: '', action: () => {}, separator: true },
    {
      label: '折叠所有文件夹',
      icon: 'collapse-all',
      action: () => collapseAll(),
    },
    {
      label: '刷新',
      icon: 'refresh-cw',
      action: () => refreshDir(),
    },
  )

  contextMenu.value = { x: event.clientX, y: event.clientY, items }
}

function handleContextMenu(event: MouseEvent): void {
  const target = event.target as HTMLElement
  if (target.closest('.panel-heading, .tree-breadcrumb')) return
  event.preventDefault()
  event.stopPropagation()
  const row = target.closest<HTMLElement>('.tree-row, .tree-child-row')
  if (row) {
    const path = row.getAttribute('data-tree-path')
    if (path) {
      const item = findItemByPath(path)
      if (item) {
        buildContextMenu(item, event)
        return
      }
    }
  }
  buildBlankContextMenu(event)
}

function findItemByPath(path: string): FsItem | null {
  const found = items.value.find((i) => i.path === path)
  if (found) return found
  for (const [, children] of childCache) {
    const child = children.find((c) => c.path === path)
    if (child) return child
  }
  return null
}

function onDragStart(item: FsItem, event: DragEvent): void {
  event.stopPropagation()
  event.dataTransfer?.setData('text/plain', item.path)
  event.dataTransfer!.effectAllowed = 'move'
}

function onDragOver(item: FsItem, event: DragEvent): void {
  event.preventDefault()
  event.stopPropagation()
  const isExternal = event.dataTransfer?.types?.includes('Files')
  event.dataTransfer!.dropEffect = isExternal ? 'copy' : 'move'
  if (item.kind === 'directory') {
    dragOverDir.value = item.path
  } else {
    const lastSlash = item.path.lastIndexOf('/')
    dragOverDir.value = lastSlash >= 0 ? item.path.slice(0, lastSlash) : currentDir.value
  }
}

function onDragLeave(event: DragEvent): void {
  event.stopPropagation()
  dragOverDir.value = null
}

async function onDrop(targetDir: string, event: DragEvent): Promise<void> {
  event.preventDefault()
  event.stopPropagation()
  dragOverDir.value = null

  // 1. Handle external file drag-and-drop upload
  if (event.dataTransfer?.files && event.dataTransfer.files.length > 0) {
    const files = Array.from(event.dataTransfer.files)
    uploadQueue.value.push(...files)
    uploadTotal.value += files.length
    const dir = targetDir !== undefined && targetDir !== null ? targetDir : workingDir.value
    processUploadQueue(dir)
    return
  }

  // 2. Handle internal node move
  const sourcePath = event.dataTransfer?.getData('text/plain')
  if (!sourcePath || sourcePath === targetDir) return
  const sourceParentDir =
    sourcePath.lastIndexOf('/') >= 0 ? sourcePath.substring(0, sourcePath.lastIndexOf('/')) : ''
  if (sourceParentDir === targetDir) return
  try {
    await moveFile(sourcePath, targetDir)
    refreshDir()
  } catch (error) {
    showToolMessage(error instanceof Error ? error.message : '移动失败')
  }
}

function onDragOverRoot(event: DragEvent): void {
  event.preventDefault()
  event.stopPropagation()
  const isExternal = event.dataTransfer?.types?.includes('Files')
  event.dataTransfer!.dropEffect = isExternal ? 'copy' : 'move'
  dragOverDir.value = currentDir.value
}

function onDragOverBreadcrumb(path: string, event: DragEvent): void {
  event.preventDefault()
  event.stopPropagation()
  const isExternal = event.dataTransfer?.types?.includes('Files')
  event.dataTransfer!.dropEffect = isExternal ? 'copy' : 'move'
  dragOverDir.value = path
}

watch(
  () => props.selectedPath,
  (path) => {
    if (!path) return
    if (suppressNavigate) {
      suppressNavigate = false
      return
    }
    const lastSlash = path.lastIndexOf('/')
    const dir = lastSlash >= 0 ? path.slice(0, lastSlash) : ''
    workingDir.value = dir
    if (dir !== currentDir.value) navigateTo(dir)
  },
)

let eventSource: EventSource | null = null
let refreshTimer: number | null = null
let sseReconnectTimer: number | null = null

function throttledRefresh() {
  if (refreshTimer) return
  refreshTimer = window.setTimeout(() => {
    refreshTimer = null
    refreshDir()
  }, 500)
}

function setupSSE() {
  if (typeof EventSource === 'undefined') return
  if (eventSource) return
  if (sseReconnectTimer) {
    clearTimeout(sseReconnectTimer)
    sseReconnectTimer = null
  }
  eventSource = new EventSource('/api/fs/events')
  eventSource.onmessage = () => {
    throttledRefresh()
  }
  eventSource.onerror = () => {
    eventSource?.close()
    eventSource = null
    if (sseReconnectTimer) {
      clearTimeout(sseReconnectTimer)
    }
    sseReconnectTimer = window.setTimeout(() => {
      sseReconnectTimer = null
      setupSSE()
    }, 5000)
  }
}

onMounted(() => {
  void getWorkspace()
    .then((res) => {
      workspaceRoot.value = res.workspace
    })
    .catch(() => {})
  loadDir('')
  setupSSE()
})

onBeforeUnmount(() => {
  controller?.abort()
  if (refreshTimer) {
    clearTimeout(refreshTimer)
    refreshTimer = null
  }
  if (sseReconnectTimer) {
    clearTimeout(sseReconnectTimer)
    sseReconnectTimer = null
  }
  if (toolMessageTimer) {
    clearTimeout(toolMessageTimer)
    toolMessageTimer = null
  }
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
})
</script>

<template>
  <div
    class="file-tree-panel"
    @dragover.prevent="onDragOverRoot"
    @dragleave="onDragLeave"
    @drop="onDrop(currentDir, $event)"
    @contextmenu="handleContextMenu"
  >
    <div class="panel-heading">
      <div>
        <h2>WORKSPACE</h2>
      </div>
      <div class="tree-toolbar">
        <input
          ref="fileInput"
          type="file"
          multiple
          class="tree-file-input"
          @change="handleUpload"
        />
        <button
          class="icon-button compact"
          type="button"
          title="上传文件"
          aria-label="上传文件"
          v-html="iconSvg('upload', 16)"
          @click="handleUploadClick"
        ></button>
        <button
          class="icon-button compact"
          type="button"
          title="新建文件"
          aria-label="新建文件"
          v-html="iconSvg('file-text', 16)"
          @click="handleCreateFile"
        ></button>
        <button
          class="icon-button compact"
          type="button"
          title="新建文件夹"
          aria-label="新建文件夹"
          v-html="iconSvg('folder', 16)"
          @click="handleCreateDir"
        ></button>
        <button
          class="icon-button compact"
          type="button"
          title="刷新"
          aria-label="刷新"
          :disabled="loading"
          v-html="iconSvg('refresh-cw', 16)"
          @click="refreshDir"
        ></button>
        <button
          class="icon-button compact"
          type="button"
          title="折叠所有文件夹"
          aria-label="折叠所有文件夹"
          v-html="iconSvg('collapse-all', 16)"
          @click="collapseAll"
        ></button>
      </div>
    </div>

    <nav class="tree-breadcrumb" aria-label="当前路径">
      <template v-for="(crumb, index) in breadcrumb" :key="crumb.path">
        <span v-if="index > 0" class="bc-sep">/</span>
        <button
          class="bc-crumb"
          :class="{
            current: index === breadcrumb.length - 1,
            'drag-over': dragOverDir !== null && dragOverDir === crumb.path,
            'bc-root': index === 0,
          }"
          type="button"
          :title="
            index === 0
              ? workspaceRoot
                ? `工作区根目录: ${workspaceRoot} (右键管理/切换)`
                : '工作区根目录 (右键管理/切换)'
              : `路径: ${crumb.path} (右键管理)`
          "
          @click="navigateTo(crumb.path)"
          @contextmenu="handleBreadcrumbContextMenu(crumb, $event)"
          @dragover.prevent="onDragOverBreadcrumb(crumb.path, $event)"
          @dragleave="onDragLeave($event)"
          @drop="onDrop(crumb.path, $event)"
        >
          {{ crumb.name }}
        </button>
      </template>
    </nav>

    <p v-if="toolMessage" class="tree-tool-message" role="alert">{{ toolMessage }}</p>

    <div
      class="tree-scroll"
      :class="{ 'drag-over': dragOverDir !== null && dragOverDir === currentDir }"
      @dragover="onDragOverRoot"
      @dragleave="onDragLeave"
      @drop="onDrop(currentDir, $event)"
    >
      <div v-if="loading && items.length === 0" class="panel-state" role="status">
        <span class="loading-ring small" aria-hidden="true"></span>
        <span>加载文件…</span>
      </div>
      <div v-else-if="errorMessage && items.length === 0" class="panel-state error" role="alert">
        <span>{{ errorMessage }}</span>
        <button class="secondary-button" type="button" @click="loadDir(currentDir)">重试</button>
      </div>
      <ul
        v-else
        class="file-tree"
        role="tree"
        aria-label="工作区文件"
      >
        <template v-for="item in items" :key="item.path">
          <li
            class="tree-node"
            role="treeitem"
            :aria-expanded="item.kind === 'directory' ? isExpanded(item.path) : undefined"
            :aria-selected="item.kind === 'file' ? selectedPath === item.path : undefined"
            :draggable="true"
            @dragstart="onDragStart(item, $event)"
            @dragover="onDragOver(item, $event)"
            @dragleave="onDragLeave($event)"
            @drop="
              onDrop(
                item.kind === 'directory'
                  ? item.path
                  : item.path.lastIndexOf('/') >= 0
                    ? item.path.slice(0, item.path.lastIndexOf('/'))
                    : currentDir,
                $event,
              )
            "
          >
            <div
              class="tree-row"
              :class="{
                selected: selectedPath === item.path,
                'drag-over': dragOverDir !== null && dragOverDir === item.path,
              }"
              :data-tree-path="item.path"
            >
              <button
                v-if="item.kind === 'directory'"
                class="tree-chevron"
                type="button"
                :aria-label="isExpanded(item.path) ? '折叠' : '展开'"
                v-html="iconSvg(isExpanded(item.path) ? 'chevron-down' : 'chevron-right', 14)"
                @click.stop="handleChevron(item)"
              ></button>
              <span v-else class="tree-chevron-spacer"></span>
              <button
                class="tree-label"
                type="button"
                :title="item.path"
                @click="handleRowClick(item)"
                @contextmenu="buildContextMenu(item, $event)"
              >
                <span
                  class="tree-icon"
                  aria-hidden="true"
                  v-html="iconSvg(fileIconName(item), 16)"
                ></span>
                <span class="tree-name">{{ item.name }}</span>
              </button>
              <button
                class="tree-delete"
                type="button"
                :aria-label="`删除 ${item.name}`"
                :title="`删除 ${item.name}`"
                v-html="iconSvg('x', 14)"
                @click.stop="handleDelete(item)"
              ></button>
            </div>

            <div v-if="item.kind === 'directory' && isExpanded(item.path)" class="tree-children">
              <div v-if="isChildLoading(item.path)" class="tree-child-loading">
                <span class="loading-ring small" aria-hidden="true"></span>
              </div>
              <div v-else-if="getChildError(item.path)" class="tree-child-error">
                <span>{{ getChildError(item.path) }}</span>
              </div>
              <ul v-else class="tree-child-list" role="group">
                <TreeChildren
                  :items="getChildren(item.path)"
                  :selected-path="selectedPath"
                  :drag-over-dir="dragOverDir"
                  :depth="0"
                  @open="emit('open', $event)"
                  @delete="handleDelete"
                  @drag-start="onDragStart"
                  @drag-over="onDragOver"
                  @drag-leave="onDragLeave"
                  @drop="onDrop"
                  @context-menu="buildContextMenu"
                  @click="handleChildClick"
                  @expand="toggleExpand"
                />
                <li v-if="getChildren(item.path).length === 0" class="tree-child-empty">空目录</li>
              </ul>
            </div>
          </li>
        </template>
        <li v-if="items.length === 0" class="tree-empty root">此目录为空</li>
      </ul>
    </div>
  </div>
  <ContextMenu
    v-if="contextMenu"
    :items="contextMenu.items"
    :x="contextMenu.x"
    :y="contextMenu.y"
    @close="closeContextMenu"
  />
</template>
