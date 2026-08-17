<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import type { TabItem } from '@/api/types'
import { iconSvg } from '@/utils/icons'
import { resolveAbsolutePath } from '@/utils/path'
import type { ContextMenuItem } from './ContextMenu.vue'
import ContextMenu from './ContextMenu.vue'

const props = defineProps<{
  tabs: TabItem[]
  activePath: string
  workspaceRoot: string
  recentFiles?: string[]
}>()

const emit = defineEmits<{
  select: [path: string]
  close: [path: string]
  closeOthers: [path: string]
  closeRight: [path: string]
  closeAll: []
  togglePin: [path: string]
  openRecent: [path: string]
}>()

const tabsContainer = ref<HTMLElement | null>(null)
const contextMenu = ref<{ x: number; y: number; items: ContextMenuItem[] } | null>(null)
const moreMenuOpen = ref(false)

function tabIcon(previewKind: string): string {
  switch (previewKind) {
    case 'markdown':
      return 'file-code'
    case 'image':
      return 'image'
    case 'text':
      return 'file-text'
    default:
      return 'file'
  }
}

function handleTabClick(tab: TabItem): void {
  emit('select', tab.path)
}

function handleCloseClick(tab: TabItem, event: MouseEvent): void {
  event.stopPropagation()
  emit('close', tab.path)
}

function handleTabAuxClick(tab: TabItem, event: MouseEvent): void {
  if (event.button === 1) {
    event.preventDefault()
    event.stopPropagation()
    emit('close', tab.path)
  }
}

function handleTabContextMenu(tab: TabItem, event: MouseEvent): void {
  event.preventDefault()
  event.stopPropagation()
  moreMenuOpen.value = false
  const items: ContextMenuItem[] = [
    {
      label: '关闭',
      icon: 'x',
      action: () => emit('close', tab.path),
    },
    {
      label: '关闭其他标签页',
      icon: 'x',
      action: () => emit('closeOthers', tab.path),
    },
    {
      label: '关闭右侧标签页',
      icon: 'x',
      action: () => emit('closeRight', tab.path),
    },
    {
      label: '关闭所有标签页',
      icon: 'x',
      action: () => emit('closeAll'),
    },
    { label: '', action: () => {}, separator: true },
    {
      label: tab.pinned ? '取消固定' : '固定标签页',
      icon: tab.pinned ? 'pin-off' : 'pin',
      action: () => emit('togglePin', tab.path),
    },
    { label: '', action: () => {}, separator: true },
    {
      label: '复制相对路径',
      icon: 'link',
      action: async () => {
        try {
          await navigator.clipboard.writeText(tab.path)
        } catch {
          // clipboard API unavailable
        }
      },
    },
    {
      label: '复制绝对路径',
      icon: 'copy',
      action: async () => {
        try {
          const abs = resolveAbsolutePath(props.workspaceRoot, tab.path)
          await navigator.clipboard.writeText(abs)
        } catch {
          // clipboard API unavailable
        }
      },
    },
  ]
  contextMenu.value = { x: event.clientX, y: event.clientY, items }
}

function handleTabBarContextMenu(event: MouseEvent): void {
  event.preventDefault()
  event.stopPropagation()
  moreMenuOpen.value = false
  const items: ContextMenuItem[] = [
    {
      label: '关闭所有标签页',
      icon: 'x',
      action: () => emit('closeAll'),
    },
  ]
  contextMenu.value = { x: event.clientX, y: event.clientY, items }
}

function handleWheel(event: WheelEvent): void {
  if (!tabsContainer.value) return
  if (Math.abs(event.deltaY) > Math.abs(event.deltaX)) {
    event.preventDefault()
    tabsContainer.value.scrollLeft += event.deltaY
  }
}

function toggleMoreMenu(event: MouseEvent): void {
  event.stopPropagation()
  moreMenuOpen.value = !moreMenuOpen.value
}

function closeMoreMenu(): void {
  moreMenuOpen.value = false
}

watch(
  () => props.activePath,
  async (newPath) => {
    if (!newPath) return
    await nextTick()
    const tabs = tabsContainer.value?.querySelectorAll<HTMLElement>('.tab-item')
    if (tabs) {
      for (const el of tabs) {
        if (el.getAttribute('data-tab-path') === newPath) {
          el.scrollIntoView?.({ behavior: 'smooth', block: 'nearest', inline: 'nearest' })
          break
        }
      }
    }
  },
  { immediate: true },
)
</script>

<template>
  <div class="document-tabs-bar" @contextmenu="handleTabBarContextMenu">
    <div
      ref="tabsContainer"
      class="tabs-scroll-container"
      role="tablist"
      aria-label="打开的文件标签"
      @wheel.passive="handleWheel"
    >
      <div
        v-for="tab in tabs"
        :key="tab.path"
        class="tab-item"
        :class="{
          active: tab.path === activePath,
          pinned: tab.pinned,
        }"
        :data-tab-path="tab.path"
        role="tab"
        :aria-selected="tab.path === activePath"
        :title="tab.path"
        tabindex="0"
        @click="handleTabClick(tab)"
        @auxclick="handleTabAuxClick(tab, $event)"
        @contextmenu="handleTabContextMenu(tab, $event)"
        @keydown.enter.prevent="handleTabClick(tab)"
        @keydown.space.prevent="handleTabClick(tab)"
      >
        <span class="tab-icon" aria-hidden="true" v-html="iconSvg(tabIcon(tab.previewKind), 13)"></span>
        <span class="tab-title">{{ tab.name }}</span>
        <span
          v-if="tab.pinned"
          class="tab-pin-icon"
          title="已固定标签 (点击取消固定)"
          aria-label="已固定标签"
          v-html="iconSvg('pin', 11)"
          @click.stop="emit('togglePin', tab.path)"
        ></span>
        <button
          v-if="!tab.pinned"
          class="tab-close-btn"
          type="button"
          :aria-label="`关闭 ${tab.name}`"
          title="关闭 (中键也可关闭)"
          v-html="iconSvg('x', 12)"
          @click="handleCloseClick(tab, $event)"
        ></button>
      </div>
    </div>

    <div class="tabs-actions">
      <div class="more-menu-wrapper">
        <button
          class="tab-action-btn"
          type="button"
          title="标签页操作"
          aria-label="标签页操作"
          :aria-expanded="moreMenuOpen"
          v-html="iconSvg('more-horizontal', 14)"
          @click="toggleMoreMenu"
        ></button>
        <div v-if="moreMenuOpen" class="more-menu-overlay" @click="closeMoreMenu"></div>
        <div v-if="moreMenuOpen" class="tabs-dropdown" role="menu">
          <button
            class="tabs-dropdown-item"
            type="button"
            role="menuitem"
            @click="
              () => {
                closeMoreMenu()
                emit('closeAll')
              }
            "
          >
            <span v-html="iconSvg('x', 13)"></span>
            <span>关闭所有标签页</span>
          </button>
          <button
            v-if="activePath"
            class="tabs-dropdown-item"
            type="button"
            role="menuitem"
            @click="
              () => {
                closeMoreMenu()
                emit('closeOthers', activePath)
              }
            "
          >
            <span v-html="iconSvg('x', 13)"></span>
            <span>关闭其他标签页</span>
          </button>
          <template v-if="recentFiles && recentFiles.length > 0">
            <div class="tabs-dropdown-divider" role="separator"></div>
            <div class="tabs-dropdown-header">
              <span v-html="iconSvg('history', 12)"></span>
              <span>最近打开</span>
            </div>
            <button
              v-for="rf in recentFiles.slice(0, 8)"
              :key="rf"
              class="tabs-dropdown-item recent-item"
              type="button"
              role="menuitem"
              :title="rf"
              @click="
                () => {
                  closeMoreMenu()
                  emit('openRecent', rf)
                }
              "
            >
              <span class="recent-name">{{ rf.split('/').pop() || rf }}</span>
              <span class="recent-path">{{ rf }}</span>
            </button>
          </template>
        </div>
      </div>
    </div>

    <ContextMenu
      v-if="contextMenu"
      :items="contextMenu.items"
      :x="contextMenu.x"
      :y="contextMenu.y"
      @close="contextMenu = null"
    />
  </div>
</template>
