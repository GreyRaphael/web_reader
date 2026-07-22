<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ICON_PATHS } from '@/utils/icons'
import { getLanguageFromPath, highlightCode } from '@/utils/prism'

const props = defineProps<{
  content: string
  path: string
}>()

const copied = ref(false)
const isHighlighting = ref(false)
const highlightedHtml = ref('')

const language = computed(() => getLanguageFromPath(props.path))
const languageBadge = computed(() => {
  const lang = language.value.toUpperCase()
  return lang === 'PLAINTEXT' ? 'CODE' : lang
})

const lineCount = computed(() => {
  if (!props.content) return 0
  return props.content.split('\n').length
})

const lineNumbersText = computed(() => {
  const count = lineCount.value
  if (count <= 0) return ''
  let result = ''
  for (let i = 1; i <= count; i++) {
    result += i + '\n'
  }
  return result
})

function updateHighlighting() {
  isHighlighting.value = true
  setTimeout(() => {
    highlightedHtml.value = highlightCode(props.content || '', language.value)
    isHighlighting.value = false
  }, 0)
}

watch([() => props.content, () => props.path], updateHighlighting, { immediate: true })

async function copyCode(): Promise<void> {
  try {
    await navigator.clipboard.writeText(props.content)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 2000)
  } catch {
    // Clipboard failed
  }
}
</script>

<template>
  <article class="code-viewer" aria-label="代码内容">
    <div class="code-viewer-header">
      <div class="code-viewer-meta">
        <span class="code-badge">{{ languageBadge }}</span>
        <span class="code-lines">{{ lineCount }} 行</span>
      </div>
      <div class="code-viewer-actions">
        <span v-if="isHighlighting" class="highlighting-spinner">高亮处理中…</span>
        <button
          type="button"
          class="code-copy-btn"
          :class="{ copied }"
          aria-label="复制代码"
          @click="copyCode"
        >
          <svg
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
            style="display: inline-block; vertical-align: -0.15em; flex-shrink: 0"
            v-html="copied ? ICON_PATHS['check'] : ICON_PATHS['clipboard']"
          ></svg>
          <span>{{ copied ? '已复制' : '复制' }}</span>
        </button>
      </div>
    </div>

    <div class="code-viewer-body scroll-surface" tabindex="0">
      <pre class="code-gutter" aria-hidden="true">{{ lineNumbersText }}</pre>
      <pre
        class="code-content"
        :class="`language-${language}`"
      ><code :class="`language-${language}`" v-html="highlightedHtml"></code></pre>
    </div>
  </article>
</template>

<style scoped>
.code-viewer {
  display: flex;
  flex-direction: column;
  width: min(100%, 1024px);
  min-height: 100%;
  margin: 0 auto;
  padding: clamp(16px, 3vw, 32px) clamp(16px, 3vw, 40px) max(48px, env(safe-area-inset-bottom));
}

.code-viewer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border: 1px solid var(--border);
  border-bottom: 0;
  border-radius: 9px 9px 0 0;
  background: var(--surface-raised);
}

.code-viewer-meta,
.code-viewer-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.code-badge {
  padding: 2px 7px;
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  background: var(--surface-muted);
  color: var(--accent-strong);
  font-family: var(--font-mono);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.03em;
}

.highlighting-spinner {
  color: var(--text-muted);
  font-family: var(--font-sans);
  font-size: 11px;
}

.code-lines {
  color: var(--text-muted);
  font-family: var(--font-mono);
  font-size: 12px;
}

.code-copy-btn {
  position: static;
  opacity: 1;
}

.code-viewer-body {
  display: flex;
  position: relative;
  border: 1px solid var(--border);
  border-radius: 0 0 9px 9px;
  background: var(--surface-muted);
  overflow-x: auto;
}

.code-gutter {
  display: block;
  margin: 0;
  padding: 0.9em 0.7em;
  border-right: 1px solid var(--border);
  background: color-mix(in srgb, var(--surface-raised) 60%, transparent);
  color: var(--text-faint);
  font-family: var(--font-mono);
  font-size: 13px;
  line-height: 1.45;
  text-align: right;
  user-select: none;
  white-space: pre;
}

.code-content {
  flex: 1;
  width: max-content;
  min-width: 0;
  margin: 0;
  padding: 0.9em 1.15em;
  background: transparent;
  overflow-x: auto;
}

.code-content code {
  padding: 0;
  background: transparent;
  color: var(--text);
  font-family: var(--font-mono);
  font-size: 13px;
  line-height: 1.45;
  tab-size: 4;
  white-space: pre;
}

@media (max-width: 600px) {
  .code-viewer {
    padding-right: 8px;
    padding-left: 8px;
  }
  .code-gutter {
    padding-right: 0.5em;
    padding-left: 0.5em;
    font-size: 12px;
  }
  .code-content code {
    font-size: 12px;
  }
}
</style>
