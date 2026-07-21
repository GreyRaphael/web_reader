<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { createDir, createFile, listDirectory, uploadFile } from '@/api/client'
import type { FsItem } from '@/api/types'
import { iconSvg } from '@/utils/icons'
import { sortFileItems } from '@/utils/sort'
import type { ContextMenuItem } from './ContextMenu.vue'
import ContextMenu from './ContextMenu.vue'
import FileTreeNode from './FileTreeNode.vue'

const props = defineProps<{ selectedPath: string }>()
const emit = defineEmits<{ open: [item: FsItem] }>()

const items = ref<FsItem[]>([])
const loading = ref(true)
const errorMessage = ref('')
const refreshToken = ref(0)
const toolMessage = ref('')
let controller: AbortController | null = null
let loadRun = 0

const contextMenu = ref<{ x: number; y: number; items: ContextMenuItem[] } | null>(null)
const menuTarget = ref<FsItem | null>(null)

const fileInput = ref<HTMLInputElement | null>(null)

async function loadRoot(refreshChildren = false): Promise<void> {
  const run = ++loadRun
  controller?.abort()
  controller = new AbortController()
  if (refreshChildren) refreshToken.value += 1
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await listDirectory('', controller.signal)
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

function refreshTree(): void {
  void loadRoot(true)
}

function getParentPath(): string {
  return ''
}

async function handleCreateFile(): Promise<void> {
  const name = prompt('文件名：')
  if (!name) return
  const parent = getParentPath()
  const fullPath = parent ? `${parent}/${name}` : name
  try {
    await createFile(fullPath)
    toolMessage.value = ''
    refreshTree()
  } catch (error) {
    toolMessage.value = error instanceof Error ? error.message : '创建文件失败'
  }
}

async function handleCreateDir(): Promise<void> {
  const name = prompt('文件夹名：')
  if (!name) return
  const parent = getParentPath()
  const fullPath = parent ? `${parent}/${name}` : name
  try {
    await createDir(fullPath)
    toolMessage.value = ''
    refreshTree()
  } catch (error) {
    toolMessage.value = error instanceof Error ? error.message : '创建文件夹失败'
  }
}

function handleUploadClick(): void {
  fileInput.value?.click()
}

async function handleUpload(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  const parent = getParentPath()
  const fullPath = parent ? `${parent}/${file.name}` : file.name
  try {
    const buffer = await file.arrayBuffer()
    await uploadFile(fullPath, buffer)
    toolMessage.value = ''
    refreshTree()
  } catch (error) {
    toolMessage.value = error instanceof Error ? error.message : '上传失败'
  } finally {
    input.value = ''
  }
}

function buildContextMenu(target: FsItem, event: MouseEvent): void {
  const items: ContextMenuItem[] = [
    {
      label: '新建文件',
      icon: 'file-text',
      action: async () => {
        const name = prompt('文件名：')
        if (!name) return
        const parent = target.kind === 'directory' ? target.path : ''
        const fullPath = parent ? `${parent}/${name}` : name
        try {
          await createFile(fullPath)
          refreshTree()
        } catch (error) {
          toolMessage.value = error instanceof Error ? error.message : '创建文件失败'
        }
      },
    },
    {
      label: '新建文件夹',
      icon: 'folder',
      action: async () => {
        const name = prompt('文件夹名：')
        if (!name) return
        const parent = target.kind === 'directory' ? target.path : ''
        const fullPath = parent ? `${parent}/${name}` : name
        try {
          await createDir(fullPath)
          refreshTree()
        } catch (error) {
          toolMessage.value = error instanceof Error ? error.message : '创建文件夹失败'
        }
      },
    },
    {
      label: '重命名',
      icon: 'pencil',
      action: async () => {
        const newName = prompt('新名称：', target.name)
        if (!newName) return
        const { renameFile } = await import('@/api/client')
        try {
          await renameFile(target.path, newName)
          refreshTree()
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
          await navigator.clipboard.writeText(target.path)
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
    { label: '', action: () => {}, separator: true },
    {
      label: '删除',
      icon: 'trash-2',
      danger: true,
      action: async () => {
        if (!confirm(`确定删除 ${target.path}？`)) return
        const { deleteFile } = await import('@/api/client')
        try {
          await deleteFile(target.path)
          refreshTree()
        } catch (error) {
          toolMessage.value = error instanceof Error ? error.message : '删除失败'
        }
      },
    },
  ]
  contextMenu.value = { x: event.clientX, y: event.clientY, items }
  menuTarget.value = target
}

function closeContextMenu(): void {
  contextMenu.value = null
  menuTarget.value = null
}

function handleContextMenu(event: MouseEvent): void {
  event.preventDefault()
  const target = event.target as HTMLElement
  const row = target.closest<HTMLElement>('.tree-row')
  if (!row) return
  const path = row.getAttribute('data-tree-path')
  if (!path) return
  const item = findItem(items.value, path)
  if (!item) return
  buildContextMenu(item, event)
}

function findItem(list: FsItem[], path: string): FsItem | null {
  for (const item of list) {
    if (item.path === path) return item
  }
  return null
}

onMounted(loadRoot)
onBeforeUnmount(() => controller?.abort())
</script>

<template>
  <div class="file-tree-panel">
    <div class="panel-heading">
      <div>
        <p class="panel-kicker">WORKSPACE</p>
        <h2>文件</h2>
      </div>
      <div class="tree-toolbar">
        <input
          ref="fileInput"
          type="file"
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
          title="刷新文件树"
          aria-label="刷新文件树"
          :disabled="loading"
          v-html="iconSvg('refresh-cw', 16)"
          @click="refreshTree"
        ></button>
      </div>
    </div>
    <p v-if="toolMessage" class="tree-tool-message" role="alert">{{ toolMessage }}</p>

    <div v-if="loading && items.length === 0" class="panel-state" role="status">
      <span class="loading-ring small" aria-hidden="true"></span>
      <span>加载文件…</span>
    </div>
    <div v-else-if="errorMessage && items.length === 0" class="panel-state error" role="alert">
      <span>{{ errorMessage }}</span>
      <button class="secondary-button" type="button" @click="loadRoot()">重试</button>
    </div>
    <ul v-else class="file-tree" role="tree" aria-label="工作区文件" @contextmenu="handleContextMenu">
      <FileTreeNode
        v-for="item in items"
        :key="item.path"
        :item="item"
        :selected-path="props.selectedPath"
        :depth="0"
        :refresh-token="refreshToken"
        @open="emit('open', $event)"
        @context-menu="buildContextMenu($event.item, $event.event)"
      />
      <li v-if="items.length === 0" class="tree-empty root">工作区为空</li>
    </ul>
  </div>
  <ContextMenu
    v-if="contextMenu"
    :items="contextMenu.items"
    :x="contextMenu.x"
    :y="contextMenu.y"
    @close="closeContextMenu"
  />
</template>