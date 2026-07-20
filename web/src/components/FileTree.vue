<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { listDirectory } from '@/api/client'
import type { FsItem } from '@/api/types'
import { sortFileItems } from '@/utils/sort'
import FileTreeNode from './FileTreeNode.vue'

const props = defineProps<{ selectedPath: string }>()
const emit = defineEmits<{ open: [item: FsItem] }>()

const items = ref<FsItem[]>([])
const loading = ref(true)
const errorMessage = ref('')
const refreshToken = ref(0)
let controller: AbortController | null = null
let loadRun = 0

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
      <button
        class="icon-button compact"
        type="button"
        title="刷新文件树"
        aria-label="刷新文件树"
        @click="refreshTree"
      >
        ↻
      </button>
    </div>

    <div v-if="loading" class="panel-state" role="status">
      <span class="loading-ring small" aria-hidden="true"></span>
      <span>加载文件…</span>
    </div>
    <div v-else-if="errorMessage" class="panel-state error" role="alert">
      <span>{{ errorMessage }}</span>
      <button class="secondary-button" type="button" @click="loadRoot()">重试</button>
    </div>
    <ul v-else class="file-tree" role="tree" aria-label="工作区文件">
      <FileTreeNode
        v-for="item in items"
        :key="item.path"
        :item="item"
        :selected-path="props.selectedPath"
        :depth="0"
        :refresh-token="refreshToken"
        @open="emit('open', $event)"
      />
      <li v-if="items.length === 0" class="tree-empty root">工作区为空</li>
    </ul>
  </div>
</template>
