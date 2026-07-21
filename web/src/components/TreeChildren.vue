<script setup lang="ts">
import { inject } from 'vue'
import type { FsItem } from '@/api/types'
import { iconSvg } from '@/utils/icons'

defineOptions({ name: 'TreeChildren' })

defineProps<{
  items: FsItem[]
  selectedPath: string
  dragOverDir: string | null
  depth: number
}>()
const emit = defineEmits<{
  open: [item: FsItem]
  delete: [item: FsItem]
  dragStart: [item: FsItem, event: DragEvent]
  dragOver: [item: FsItem, event: DragEvent]
  dragLeave: [event: DragEvent]
  drop: [targetDir: string, event: DragEvent]
  contextMenu: [item: FsItem, event: MouseEvent]
  click: [item: FsItem]
  expand: [dir: string]
}>()

const expanded = inject('expandedDirs') as Set<string>
const cache = inject('childCache') as Map<string, FsItem[]>
const loadChildren = inject('loadChildren') as (dir: string) => Promise<void>

function isExpanded(dir: string): boolean {
  return expanded.has(dir)
}

function getChildren(dir: string): FsItem[] {
  return cache.get(dir) ?? []
}

function toggleExpand(dir: string): void {
  if (expanded.has(dir)) {
    expanded.delete(dir)
  } else {
    expanded.add(dir)
    if (!cache.has(dir)) void loadChildren(dir)
  }
}

function onDragStart(item: FsItem, event: DragEvent): void {
  event.stopPropagation()
  emit('dragStart', item, event)
}

function onDragOver(item: FsItem, event: DragEvent): void {
  event.preventDefault()
  event.stopPropagation()
  emit('dragOver', item, event)
}

function onDragLeave(_item: FsItem, event: DragEvent): void {
  event.stopPropagation()
  emit('dragLeave', event)
}

function onDrop(item: FsItem, event: DragEvent): void {
  event.preventDefault()
  event.stopPropagation()
  const targetDir = item.kind === 'directory' ? item.path : (item.path.lastIndexOf('/') >= 0 ? item.path.slice(0, item.path.lastIndexOf('/')) : '')
  emit('drop', targetDir, event)
}

function onContextMenu(item: FsItem, event: MouseEvent): void {
  emit('contextMenu', item, event)
}

function handleClick(item: FsItem): void {
  emit('click', item)
}

function iconName(item: FsItem): string {
  if (item.kind === 'directory') return 'folder'
  switch (item.previewKind) {
    case 'markdown': return 'file-code'
    case 'image': return 'image'
    case 'text': return 'file-text'
    default: return 'file'
  }
}
</script>

<template>
  <template v-for="item in items" :key="item.path">
    <li
      class="tree-child-li"
      :class="{ 'drag-over': dragOverDir !== null && dragOverDir === item.path }"
      :draggable="true"
      @dragstart="onDragStart(item, $event)"
      @dragover="onDragOver(item, $event)"
      @dragleave="onDragLeave(item, $event)"
      @drop="onDrop(item, $event)"
    >
      <button
        v-if="item.kind === 'directory'"
        class="tree-chevron tree-chevron-sm"
        type="button"
        :aria-label="isExpanded(item.path) ? '折叠' : '展开'"
        v-html="iconSvg(isExpanded(item.path) ? 'chevron-down' : 'chevron-right', 12)"
        @click.stop="toggleExpand(item.path)"
      ></button>
      <span v-else class="tree-chevron-spacer"></span>
      <button
        class="tree-child-row"
        :class="{ selected: selectedPath === item.path }"
        type="button"
        :title="item.path"
        :data-tree-path="item.path"
        @click="handleClick(item)"
        @contextmenu="onContextMenu(item, $event)"
      >
        <span class="tree-icon" aria-hidden="true" v-html="iconSvg(iconName(item), 12)"></span>
        <span class="tree-name">{{ item.name }}</span>
      </button>
      <button
        class="tree-delete"
        type="button"
        :aria-label="`删除 ${item.name}`"
        :title="`删除 ${item.name}`"
        v-html="iconSvg('x', 14)"
        @click.stop="emit('delete', item)"
      ></button>
    </li>
    <li v-if="item.kind === 'directory' && isExpanded(item.path)" class="tree-child-expanded">
      <ul class="tree-child-list">
        <TreeChildren
          :items="getChildren(item.path)"
          :selected-path="selectedPath"
          :drag-over-dir="dragOverDir"
          :depth="depth + 1"
          @open="(item: FsItem) => emit('open', item)"
          @delete="(item: FsItem) => emit('delete', item)"
          @drag-start="(item: FsItem, ev: DragEvent) => emit('dragStart', item, ev)"
          @drag-over="(item: FsItem, ev: DragEvent) => emit('dragOver', item, ev)"
          @drag-leave="(ev: DragEvent) => emit('dragLeave', ev)"
          @drop="(dir: string, ev: DragEvent) => emit('drop', dir, ev)"
          @context-menu="(item: FsItem, ev: MouseEvent) => emit('contextMenu', item, ev)"
          @click="(item: FsItem) => emit('click', item)"
          @expand="(dir: string) => emit('expand', dir)"
        />
      </ul>
    </li>
  </template>
</template>