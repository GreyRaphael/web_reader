<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { listDirectory } from '@/api/client'
import type { FsItem } from '@/api/types'
import { sortFileItems } from '@/utils/sort'

const props = defineProps<{
  item: FsItem
  selectedPath: string
  depth: number
  refreshToken: number
}>()
const emit = defineEmits<{ open: [item: FsItem] }>()

const expanded = ref(false)
const loading = ref(false)
const loaded = ref(false)
const errorMessage = ref('')
const children = ref<FsItem[]>([])
let controller: AbortController | null = null
let loadRun = 0

const icon = computed(() => {
  if (props.item.kind === 'directory') return expanded.value ? '▾' : '▸'
  switch (props.item.previewKind) {
    case 'markdown':
      return 'M'
    case 'image':
      return '◇'
    case 'text':
      return '≡'
    default:
      return '·'
  }
})

async function loadChildren(): Promise<void> {
  if (loaded.value || loading.value) return
  const run = ++loadRun
  controller?.abort()
  controller = new AbortController()
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await listDirectory(props.item.path, controller.signal)
    if (run !== loadRun) return
    children.value = sortFileItems(response.items)
    loaded.value = true
  } catch (error) {
    if (error instanceof Error && error.name === 'AbortError') return
    if (run !== loadRun) return
    errorMessage.value = error instanceof Error ? error.message : '目录加载失败'
  } finally {
    if (run === loadRun) loading.value = false
  }
}

async function activate(): Promise<void> {
  if (props.item.kind === 'directory') {
    expanded.value = !expanded.value
    if (expanded.value) await loadChildren()
    return
  }
  emit('open', props.item)
}

async function retry(): Promise<void> {
  loaded.value = false
  await loadChildren()
}

function visibleRows(target: HTMLElement): HTMLButtonElement[] {
  const tree = target.closest('[role="tree"]')
  return tree ? Array.from(tree.querySelectorAll<HTMLButtonElement>('.tree-row')) : []
}

async function handleKeydown(event: KeyboardEvent): Promise<void> {
  const target = event.currentTarget
  if (!(target instanceof HTMLButtonElement)) return
  const rows = visibleRows(target)
  const index = rows.indexOf(target)

  if (
    event.key === 'ArrowDown' ||
    event.key === 'ArrowUp' ||
    event.key === 'Home' ||
    event.key === 'End'
  ) {
    event.preventDefault()
    const nextIndex =
      event.key === 'Home'
        ? 0
        : event.key === 'End'
          ? rows.length - 1
          : Math.min(rows.length - 1, Math.max(0, index + (event.key === 'ArrowDown' ? 1 : -1)))
    rows[nextIndex]?.focus()
    return
  }

  if (props.item.kind !== 'directory') return
  if (event.key === 'ArrowRight') {
    event.preventDefault()
    if (!expanded.value) await activate()
    else {
      await nextTick()
      const ownNode = target.closest('.tree-node')
      ownNode?.querySelector<HTMLButtonElement>('.tree-group > .tree-node > .tree-row')?.focus()
    }
  } else if (event.key === 'ArrowLeft' && expanded.value) {
    event.preventDefault()
    expanded.value = false
  }
}

watch(
  () => props.refreshToken,
  () => {
    loadRun += 1
    controller?.abort()
    loaded.value = false
    loading.value = false
    errorMessage.value = ''
    if (expanded.value) void loadChildren()
  },
)

onBeforeUnmount(() => {
  loadRun += 1
  controller?.abort()
})
</script>

<template>
  <li
    class="tree-node"
    role="treeitem"
    :aria-expanded="item.kind === 'directory' ? expanded : undefined"
    :aria-selected="item.kind === 'file' ? selectedPath === item.path : undefined"
  >
    <button
      class="tree-row"
      :class="{ selected: selectedPath === item.path }"
      type="button"
      :style="{ '--tree-depth': depth }"
      :title="item.path"
      @click="activate"
      @keydown="handleKeydown"
    >
      <span class="tree-icon" :class="`kind-${item.kind}`" aria-hidden="true">{{ icon }}</span>
      <span class="tree-name">{{ item.name }}</span>
      <span v-if="loading" class="tiny-spinner" aria-label="加载中"></span>
    </button>

    <div v-if="expanded && errorMessage" class="tree-error" role="alert">
      <span>{{ errorMessage }}</span>
      <button type="button" @click="retry">重试</button>
    </div>
    <ul v-if="expanded && loaded" class="tree-group" role="group">
      <FileTreeNode
        v-for="child in children"
        :key="child.path"
        :item="child"
        :selected-path="selectedPath"
        :depth="depth + 1"
        :refresh-token="refreshToken"
        @open="emit('open', $event)"
      />
      <li v-if="children.length === 0" class="tree-empty">空目录</li>
    </ul>
  </li>
</template>
