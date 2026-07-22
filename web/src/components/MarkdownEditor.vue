<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { highlightCode } from '@/utils/prism'

const props = defineProps<{
  content: string
}>()

const emit = defineEmits<{
  'update:content': [value: string]
  save: []
  scroll: [event: Event]
}>()

const textareaRef = ref<HTMLTextAreaElement | null>(null)
const highlightRef = ref<HTMLPreElement | null>(null)
const gutterRef = ref<HTMLPreElement | null>(null)

const localContent = ref(props.content)

watch(
  () => props.content,
  (newVal) => {
    if (newVal !== localContent.value) {
      localContent.value = newVal
    }
  },
)

function handleInput(event: Event) {
  const target = event.target as HTMLTextAreaElement
  localContent.value = target.value
  emit('update:content', target.value)
}

const lineCount = computed(() => {
  if (!localContent.value) return 1
  return localContent.value.split('\n').length
})

const lineNumbersText = computed(() => {
  const count = lineCount.value
  let res = ''
  for (let i = 1; i <= count; i++) {
    res += i + '\n'
  }
  return res
})

const highlightedMarkdownHtml = computed(() => {
  const text = localContent.value + (localContent.value.endsWith('\n') ? ' ' : '')
  return highlightCode(text, 'markdown')
})

function syncScroll(event?: Event) {
  if (!textareaRef.value) return
  const { scrollTop, scrollLeft } = textareaRef.value

  if (highlightRef.value) {
    highlightRef.value.scrollTop = scrollTop
    highlightRef.value.scrollLeft = scrollLeft
  }
  if (gutterRef.value) {
    gutterRef.value.scrollTop = scrollTop
  }
  if (event) {
    emit('scroll', event)
  }
}

function getTextareaElement(): HTMLTextAreaElement | null {
  return textareaRef.value
}

function handleKeydown(event: KeyboardEvent) {
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
    event.preventDefault()
    emit('save')
    return
  }

  if (event.key === 'Tab') {
    event.preventDefault()
    const textarea = textareaRef.value
    if (!textarea) return
    const start = textarea.selectionStart
    const end = textarea.selectionEnd
    const val = localContent.value
    localContent.value = val.substring(0, start) + '  ' + val.substring(end)
    emit('update:content', localContent.value)

    setTimeout(() => {
      textarea.selectionStart = textarea.selectionEnd = start + 2
    }, 0)
  }
}

defineExpose({ getTextareaElement, syncScroll })
</script>

<template>
  <div class="markdown-editor scroll-surface">
    <pre ref="gutterRef" class="editor-gutter" aria-hidden="true">{{ lineNumbersText }}</pre>
    <div class="editor-workspace">
      <textarea
        ref="textareaRef"
        class="editor-textarea"
        :value="localContent"
        spellcheck="false"
        placeholder="在此输入 Markdown 内容…"
        @input="handleInput"
        @scroll="syncScroll($event)"
        @keydown="handleKeydown"
      ></textarea>
      <pre
        ref="highlightRef"
        class="editor-highlight language-markdown"
        aria-hidden="true"
      ><code class="language-markdown" v-html="highlightedMarkdownHtml"></code></pre>
    </div>
  </div>
</template>

<style scoped>
.markdown-editor {
  display: flex;
  position: relative;
  width: 100%;
  height: 100%;
  min-height: 400px;
  border: 1px solid var(--border);
  border-radius: 9px;
  background: var(--surface-muted);
  overflow: hidden;
}

.editor-gutter {
  display: block;
  flex-shrink: 0;
  margin: 0;
  padding: 1.15em 0.8em;
  border-right: 1px solid var(--border);
  background: color-mix(in srgb, var(--surface-raised) 60%, transparent);
  color: var(--text-faint);
  font-family: var(--font-mono);
  font-size: 13px;
  line-height: 1.65;
  text-align: right;
  user-select: none;
  white-space: pre;
  overflow: hidden;
}

.editor-workspace {
  position: relative;
  flex: 1;
  width: 100%;
  height: 100%;
  min-width: 0;
  overflow: hidden;
}

.editor-textarea,
.editor-highlight {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  margin: 0;
  padding: 1.15em 1.25em;
  border: none;
  outline: none;
  box-sizing: border-box;
  font-family: var(--font-mono);
  font-size: 13px;
  line-height: 1.65;
  tab-size: 2;
  white-space: pre-wrap;
  word-break: break-word;
  overflow-wrap: break-word;
  overflow: auto;
}

.editor-textarea {
  z-index: 2;
  color: transparent;
  background: transparent;
  caret-color: var(--text);
  resize: none;
}

.editor-textarea::selection {
  background: color-mix(in srgb, var(--accent) 30%, transparent);
}

.editor-highlight {
  z-index: 1;
  background: transparent;
  pointer-events: none;
}

.editor-highlight code {
  padding: 0;
  background: transparent;
  color: var(--text);
  font-family: inherit;
  font-size: inherit;
  line-height: inherit;
  white-space: inherit;
  word-break: inherit;
  overflow-wrap: inherit;
}
</style>
