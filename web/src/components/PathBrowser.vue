<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { browseDirectories } from '@/api/client'
import type { BrowseDirEntry } from '@/api/types'
import { iconSvg } from '@/utils/icons'

const props = defineProps<{ initialPath: string }>()
const emit = defineEmits<{
  select: [path: string]
  cancel: []
}>()

const currentPath = ref('')
const dirs = ref<BrowseDirEntry[]>([])
const loading = ref(false)
const errorMsg = ref('')
let controller: AbortController | null = null

const breadcrumbs = computed(() => {
  if (!currentPath.value) return []
  const parts = currentPath.value.split('/')
  const crumbs: BrowseDirEntry[] = [{ name: '/', path: '/' }]
  let acc = ''
  for (const part of parts) {
    if (!part) continue
    acc = acc ? `${acc}/${part}` : `/${part}`
    crumbs.push({ name: part, path: acc })
  }
  return crumbs
})

async function load(path: string) {
  controller?.abort()
  controller = new AbortController()
  loading.value = true
  errorMsg.value = ''
  try {
    const res = await browseDirectories(path)
    currentPath.value = res.path
    dirs.value = res.dirs
  } catch (err) {
    if (err instanceof Error && err.name === 'AbortError') return
    errorMsg.value = err instanceof Error ? err.message : '路径加载失败'
    dirs.value = []
  } finally {
    loading.value = false
  }
}

function enter(dir: BrowseDirEntry) {
  void load(dir.path)
}

function goUp() {
  const parts = currentPath.value.split('/').filter(Boolean)
  if (parts.length === 0) return
  parts.pop()
  void load(parts.length === 0 ? '/' : '/' + parts.join('/'))
}

function confirm() {
  emit('select', currentPath.value)
}

onMounted(() => {
  void load(props.initialPath || '~')
})

onBeforeUnmount(() => controller?.abort())
</script>

<template>
  <div class="path-browser">
    <div class="browser-toolbar">
      <button
        class="icon-button compact"
        type="button"
        title="上一级"
        aria-label="上一级"
        :disabled="currentPath === '/' || loading"
        v-html="iconSvg('chevrons-up', 16)"
        @click="goUp"
      ></button>
      <nav class="browser-crumbs" aria-label="当前路径">
        <template v-for="(crumb, index) in breadcrumbs" :key="crumb.path">
          <span v-if="index > 0 && breadcrumbs[index - 1]?.name !== '/'" class="crumb-sep">/</span>
          <button
            class="crumb-btn"
            type="button"
            :class="{ current: index === breadcrumbs.length - 1 }"
            @click="enter(crumb)"
          >
            {{ crumb.name }}
          </button>
        </template>
      </nav>
    </div>

    <div class="browser-list scroll-surface">
      <div v-if="loading" class="browser-state" role="status">
        <span class="loading-ring small" aria-hidden="true"></span>
        <span>加载目录…</span>
      </div>
      <div v-else-if="errorMsg" class="browser-state error" role="alert">
        <span>{{ errorMsg }}</span>
      </div>
      <div v-else-if="dirs.length === 0" class="browser-state empty">
        <span>此目录无子目录</span>
      </div>
      <ul v-else class="dir-list" role="list">
        <li v-for="dir in dirs" :key="dir.path">
          <button class="dir-item" type="button" :title="dir.path" @click="enter(dir)">
            <span class="dir-icon" aria-hidden="true" v-html="iconSvg('folder', 16)"></span>
            <span class="dir-name">{{ dir.name }}</span>
          </button>
        </li>
      </ul>
    </div>

    <div class="browser-footer">
      <span class="browser-current" :title="currentPath">{{ currentPath }}</span>
      <div class="browser-actions">
        <button class="secondary-button" type="button" @click="emit('cancel')">取消</button>
        <button class="primary-button" type="button" @click="confirm">选择此目录</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.path-browser {
  display: flex;
  flex-direction: column;
  gap: 10px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface);
  overflow: hidden;
  max-height: 320px;
}

.browser-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  border-bottom: 1px solid var(--border);
  background: var(--surface-raised);
}

.browser-crumbs {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 2px;
  overflow: hidden;
}

.crumb-btn {
  padding: 2px 6px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--text-muted);
  font-size: 12px;
  cursor: pointer;
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.crumb-btn:hover {
  background: var(--surface-hover);
  color: var(--text);
}

.crumb-btn.current {
  color: var(--accent-strong);
  font-weight: 600;
}

.crumb-sep {
  color: var(--text-faint);
  font-size: 12px;
}

.browser-list {
  flex: 1;
  min-height: 120px;
  overflow-y: auto;
  padding: 4px 0;
}

.browser-state {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 24px 12px;
  color: var(--text-muted);
  font-size: 13px;
}

.browser-state.error {
  color: var(--danger);
}

.dir-list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.dir-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 6px 12px;
  border: none;
  background: transparent;
  color: var(--text);
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  transition: background-color 120ms ease;
}

.dir-item:hover {
  background: var(--surface-hover);
}

.dir-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.browser-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 8px 12px;
  border-top: 1px solid var(--border);
  background: var(--surface-raised);
}

.browser-current {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-muted);
  font-family: var(--font-mono);
  font-size: 11px;
}

.browser-actions {
  display: flex;
  gap: 8px;
}

.browser-actions .primary-button,
.browser-actions .secondary-button {
  min-height: 32px;
  padding: 0 12px;
  font-size: 12px;
}
</style>
