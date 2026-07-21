<script setup lang="ts">
import { defineAsyncComponent, ref } from 'vue'
import type { FsItem, TextResponse } from '@/api/types'
import type { ResolvedTheme } from '@/composables/useTheme'
import type { MarkdownHeading } from '@/markdown/render'
import { formatBytes } from '@/utils/format'

import { getPreviewMode } from '@/utils/preview'
import ImageViewer from './ImageViewer.vue'
import type MarkdownViewerComponent from './MarkdownViewer.vue'
import TextViewer from './TextViewer.vue'
import UnsupportedViewer from './UnsupportedViewer.vue'

const MarkdownViewer = defineAsyncComponent(() => import('./MarkdownViewer.vue'))

defineProps<{
  item: FsItem | null
  text: TextResponse | null
  loading: boolean
  error: string
  theme: ResolvedTheme
}>()
const emit = defineEmits<{
  headings: [items: MarkdownHeading[]]
  activeHeading: [id: string]
  openPath: [path: string, hash: string]
  retry: []
}>()

const markdownViewer = ref<InstanceType<typeof MarkdownViewerComponent> | null>(null)

function scrollToHeading(id: string): void {
  markdownViewer.value?.scrollToHeading(id)
}

function forwardOpenPath(path: string, hash: string): void {
  emit('openPath', path, hash)
}

defineExpose({ scrollToHeading })
</script>

<template>
  <section class="preview-pane" aria-label="文件预览">
    <div v-if="item" class="preview-header">
      <div class="preview-title-block">
        <div class="file-type-badge">{{ getPreviewMode(item) === 'markdown' ? 'MD' : getPreviewMode(item) === 'image' ? 'IMG' : getPreviewMode(item) === 'text' ? 'TXT' : '?' }}</div>
        <div>
          <h1>{{ item.name }}</h1>
          <p>{{ item.path }}<span v-if="item.size !== undefined"> · {{ formatBytes(item.size) }}</span></p>
        </div>
      </div>
    </div>

    <div class="preview-scroll">
      <div v-if="loading" class="preview-state" role="status">
        <span class="loading-ring" aria-hidden="true"></span>
        <p>正在读取文件…</p>
      </div>
      <div v-else-if="error" class="preview-state error" role="alert">
        <div class="state-icon">!</div>
        <h2>无法打开文件</h2>
        <p>{{ error }}</p>
        <button class="secondary-button" type="button" @click="emit('retry')">重试</button>
      </div>
      <div v-else-if="!item" class="welcome-state">
        <div class="welcome-glyph" aria-hidden="true">R</div>
        <p class="eyebrow">WEB READER</p>
        <h1>从文件树选择内容</h1>
        <p>支持 Markdown、普通文本、日志与常见图片格式。</p>
        <div class="welcome-shortcuts">
          <span><kbd>☰</kbd> 文件</span>
          <span><kbd>☷</kbd> 大纲</span>
        </div>
      </div>
      <template v-else>
        <MarkdownViewer
          v-if="getPreviewMode(item!) === 'markdown' && text"
          ref="markdownViewer"
          :content="text!.content"
          :current-path="item!.path"
          :theme="theme"
          @headings="emit('headings', $event)"
          @active-heading="emit('activeHeading', $event)"
          @open-path="forwardOpenPath"
        />
        <TextViewer
          v-else-if="getPreviewMode(item!) === 'text' && text"
          :content="text!.content"
          :encoding="text!.encoding"
        />
        <ImageViewer v-else-if="getPreviewMode(item!) === 'image'" :item="item!" />
        <UnsupportedViewer v-else :item="item!" />
      </template>
    </div>
  </section>
</template>
