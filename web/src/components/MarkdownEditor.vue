<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { createDir, uploadFile } from '@/api/client'
import { iconSvg } from '@/utils/icons'

const props = defineProps<{
  content: string
  currentPath?: string
}>()

const emit = defineEmits<{
  'update:content': [value: string]
  save: []
  scroll: [event: Event]
}>()

const textareaRef = ref<HTMLTextAreaElement | null>(null)
const gutterRef = ref<HTMLPreElement | null>(null)
const fileInputRef = ref<HTMLInputElement | null>(null)

const localContent = ref(props.content)
const uploadingImage = ref(false)
const uploadStatus = ref('')
const uploadIsError = ref(false)

// ---------------- Undo / Redo History Manager ----------------

interface EditorState {
  content: string
  selectionStart: number
  selectionEnd: number
  scrollTop: number
  scrollLeft: number
}

const undoStack = ref<EditorState[]>([])
const redoStack = ref<EditorState[]>([])
const MAX_HISTORY = 100

const canUndo = computed(() => undoStack.value.length > 0)
const canRedo = computed(() => redoStack.value.length > 0)

let typingTimer: number | null = null
let beforeTypingState: EditorState | null = null

function recordBeforeChange(): void {
  commitCurrentTyping()
  const textarea = textareaRef.value
  const state: EditorState = {
    content: localContent.value,
    selectionStart: textarea?.selectionStart ?? 0,
    selectionEnd: textarea?.selectionEnd ?? 0,
    scrollTop: textarea?.scrollTop ?? 0,
    scrollLeft: textarea?.scrollLeft ?? 0,
  }
  undoStack.value.push(state)
  if (undoStack.value.length > MAX_HISTORY) {
    undoStack.value.shift()
  }
  redoStack.value = []
}

function commitCurrentTyping(): void {
  if (typingTimer !== null) {
    window.clearTimeout(typingTimer)
    typingTimer = null
  }
  if (beforeTypingState && beforeTypingState.content !== localContent.value) {
    undoStack.value.push(beforeTypingState)
    if (undoStack.value.length > MAX_HISTORY) {
      undoStack.value.shift()
    }
    redoStack.value = []
  }
  beforeTypingState = null
}

function undo(): void {
  commitCurrentTyping()
  if (undoStack.value.length === 0) return
  const textarea = textareaRef.value
  const currentState: EditorState = {
    content: localContent.value,
    selectionStart: textarea?.selectionStart ?? 0,
    selectionEnd: textarea?.selectionEnd ?? 0,
    scrollTop: textarea?.scrollTop ?? 0,
    scrollLeft: textarea?.scrollLeft ?? 0,
  }
  redoStack.value.push(currentState)

  const previousState = undoStack.value.pop()!
  localContent.value = previousState.content
  emit('update:content', previousState.content)

  void nextTick(() => {
    if (textarea) {
      textarea.focus({ preventScroll: true })
      textarea.setSelectionRange(previousState.selectionStart, previousState.selectionEnd)
      textarea.scrollTop = previousState.scrollTop
      textarea.scrollLeft = previousState.scrollLeft
      syncScroll()
    }
  })
}

function redo(): void {
  commitCurrentTyping()
  if (redoStack.value.length === 0) return
  const textarea = textareaRef.value
  const currentState: EditorState = {
    content: localContent.value,
    selectionStart: textarea?.selectionStart ?? 0,
    selectionEnd: textarea?.selectionEnd ?? 0,
    scrollTop: textarea?.scrollTop ?? 0,
    scrollLeft: textarea?.scrollLeft ?? 0,
  }
  undoStack.value.push(currentState)

  const nextState = redoStack.value.pop()!
  localContent.value = nextState.content
  emit('update:content', nextState.content)

  void nextTick(() => {
    if (textarea) {
      textarea.focus({ preventScroll: true })
      textarea.setSelectionRange(nextState.selectionStart, nextState.selectionEnd)
      textarea.scrollTop = nextState.scrollTop
      textarea.scrollLeft = nextState.scrollLeft
      syncScroll()
    }
  })
}

watch(
  () => props.content,
  (newVal) => {
    if (newVal !== localContent.value) {
      const textarea = textareaRef.value
      const savedScrollTop = textarea?.scrollTop
      const savedScrollLeft = textarea?.scrollLeft
      localContent.value = newVal
      undoStack.value = []
      redoStack.value = []
      beforeTypingState = null
      if (textarea && savedScrollTop !== undefined) {
        void nextTick(() => {
          textarea.scrollTop = savedScrollTop
          textarea.scrollLeft = savedScrollLeft || 0
          syncScroll()
        })
      }
    }
  },
)

function handleInput(event: Event) {
  const target = event.target as HTMLTextAreaElement
  if (!beforeTypingState) {
    beforeTypingState = {
      content: localContent.value,
      selectionStart: target.selectionStart,
      selectionEnd: target.selectionEnd,
      scrollTop: target.scrollTop,
      scrollLeft: target.scrollLeft,
    }
  }
  localContent.value = target.value
  emit('update:content', target.value)

  if (typingTimer !== null) {
    window.clearTimeout(typingTimer)
  }
  typingTimer = window.setTimeout(() => {
    commitCurrentTyping()
  }, 400)
}

onBeforeUnmount(() => {
  if (typingTimer !== null) {
    window.clearTimeout(typingTimer)
    typingTimer = null
  }
})

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

function syncScroll(event?: Event) {
  if (!textareaRef.value) return
  const { scrollTop } = textareaRef.value

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

// ---------------- Text Manipulation Helpers ----------------

function toggleWrapSelection(before: string, after = before, placeholder = '文本'): void {
  const textarea = textareaRef.value
  if (!textarea) return
  recordBeforeChange()
  const start = textarea.selectionStart
  const end = textarea.selectionEnd
  const savedScrollTop = textarea.scrollTop
  const savedScrollLeft = textarea.scrollLeft
  const val = localContent.value
  const hasSelection = start !== end
  const selected = hasSelection ? val.substring(start, end) : placeholder

  const isItalic = before === '*' && after === '*'

  // 1. Check if the selection itself starts with `before` and ends with `after`
  const isWrappedInsideSelection = (() => {
    if (!hasSelection) return false
    if (selected.length < before.length + after.length) return false
    if (!selected.startsWith(before) || !selected.endsWith(after)) return false
    if (isItalic) {
      if (selected.startsWith('**') && !selected.startsWith('***')) return false
      if (selected.endsWith('**') && !selected.endsWith('***')) return false
    }
    return true
  })()

  if (isWrappedInsideSelection) {
    const unwrapped = selected.substring(before.length, selected.length - after.length)
    localContent.value = val.substring(0, start) + unwrapped + val.substring(end)
    emit('update:content', localContent.value)

    void nextTick(() => {
      textarea.focus({ preventScroll: true })
      textarea.setSelectionRange(start, start + unwrapped.length)
      textarea.scrollTop = savedScrollTop
      textarea.scrollLeft = savedScrollLeft
      syncScroll()
    })
    return
  }

  // 2. Check if delimiters are immediately outside the selection
  const isWrappedOutsideSelection = (() => {
    if (start < before.length || end + after.length > val.length) return false
    const textBefore = val.substring(start - before.length, start)
    const textAfter = val.substring(end, end + after.length)
    if (textBefore !== before || textAfter !== after) return false
    if (isItalic) {
      const charBefore2 = start >= 2 ? val[start - 2] : ''
      const charAfter2 = end + 1 < val.length ? val[end + 1] : ''
      if (charBefore2 === '*' || charAfter2 === '*') return false
    }
    return true
  })()

  if (isWrappedOutsideSelection) {
    const newStart = start - before.length
    const unwrapped = val.substring(start, end)
    localContent.value = val.substring(0, newStart) + unwrapped + val.substring(end + after.length)
    emit('update:content', localContent.value)

    void nextTick(() => {
      textarea.focus({ preventScroll: true })
      textarea.setSelectionRange(newStart, newStart + unwrapped.length)
      textarea.scrollTop = savedScrollTop
      textarea.scrollLeft = savedScrollLeft
      syncScroll()
    })
    return
  }

  // 3. Otherwise, apply formatting
  const replacement = `${before}${selected}${after}`
  localContent.value = val.substring(0, start) + replacement + val.substring(end)
  emit('update:content', localContent.value)

  void nextTick(() => {
    textarea.focus({ preventScroll: true })
    if (hasSelection) {
      textarea.setSelectionRange(start + before.length, start + before.length + selected.length)
    } else {
      textarea.setSelectionRange(start + before.length, start + before.length + placeholder.length)
    }
    textarea.scrollTop = savedScrollTop
    textarea.scrollLeft = savedScrollLeft
    syncScroll()
  })
}

function prefixLines(prefix: string): void {
  const textarea = textareaRef.value
  if (!textarea) return
  recordBeforeChange()
  const start = textarea.selectionStart
  const end = textarea.selectionEnd
  const savedScrollTop = textarea.scrollTop
  const savedScrollLeft = textarea.scrollLeft
  const val = localContent.value

  const lineStart = val.lastIndexOf('\n', start - 1) + 1
  const rawLineEnd = val.indexOf('\n', end)
  const lineEnd = rawLineEnd === -1 ? val.length : rawLineEnd

  const selectedLines = val.substring(lineStart, lineEnd).split('\n')
  const allPrefixed = selectedLines.every((l) => l.startsWith(prefix))

  const newLines = selectedLines.map((l) => {
    if (allPrefixed) {
      return l.slice(prefix.length)
    }
    return `${prefix}${l}`
  })

  const replacement = newLines.join('\n')
  localContent.value = val.substring(0, lineStart) + replacement + val.substring(lineEnd)
  emit('update:content', localContent.value)

  void nextTick(() => {
    textarea.focus({ preventScroll: true })
    textarea.setSelectionRange(lineStart, lineStart + replacement.length)
    textarea.scrollTop = savedScrollTop
    textarea.scrollLeft = savedScrollLeft
    syncScroll()
  })
}

function insertTemplate(template: string, selectRange?: [number, number]): void {
  const textarea = textareaRef.value
  if (!textarea) return
  recordBeforeChange()
  const start = textarea.selectionStart
  const end = textarea.selectionEnd
  const savedScrollTop = textarea.scrollTop
  const savedScrollLeft = textarea.scrollLeft
  const val = localContent.value

  const needsLeadingNewline = start > 0 && val[start - 1] !== '\n'
  const textToInsert = (needsLeadingNewline ? '\n' : '') + template

  localContent.value = val.substring(0, start) + textToInsert + val.substring(end)
  emit('update:content', localContent.value)

  void nextTick(() => {
    textarea.focus({ preventScroll: true })
    const offset = needsLeadingNewline ? 1 : 0
    if (selectRange) {
      textarea.setSelectionRange(start + offset + selectRange[0], start + offset + selectRange[1])
    } else {
      textarea.setSelectionRange(start + textToInsert.length, start + textToInsert.length)
    }
    textarea.scrollTop = savedScrollTop
    textarea.scrollLeft = savedScrollLeft
    syncScroll()
  })
}

// ---------------- Toolbar Actions ----------------

function handleBold(): void {
  toggleWrapSelection('**', '**', '加粗文本')
}

function handleItalic(): void {
  toggleWrapSelection('*', '*', '斜体文本')
}

function handleStrikethrough(): void {
  toggleWrapSelection('~~', '~~', '删除线文本')
}

function handleTaskList(): void {
  prefixLines('- [ ] ')
}

function handleInlineCode(): void {
  toggleWrapSelection('`', '`', '代码')
}

function handleCodeBlock(): void {
  insertTemplate('```js\n// 在此输入代码\n```\n', [6, 17])
}

function handleLink(): void {
  const textarea = textareaRef.value
  const start = textarea?.selectionStart ?? 0
  const end = textarea?.selectionEnd ?? 0
  const selected = localContent.value.substring(start, end)
  if (selected) {
    toggleWrapSelection('[', '](url)', selected)
  } else {
    insertTemplate('[链接文本](https://example.com)', [1, 5])
  }
}

function handleWikiLink(): void {
  const textarea = textareaRef.value
  const start = textarea?.selectionStart ?? 0
  const end = textarea?.selectionEnd ?? 0
  const selected = localContent.value.substring(start, end)
  if (selected) {
    toggleWrapSelection('[[', ']]', selected)
  } else {
    insertTemplate('[[文档名]]', [2, 5])
  }
}

function handleTable(): void {
  const tableTemplate =
    '| 标题 1 | 标题 2 | 标题 3 |\n| :--- | :--- | :--- |\n| 单元格 1 | 单元格 2 | 单元格 3 |\n| 单元格 4 | 单元格 5 | 单元格 6 |\n'
  insertTemplate(tableTemplate)
}

// ---------------- Image Upload & Paste ----------------

async function uploadAndInsertImage(file: File): Promise<void> {
  try {
    uploadingImage.value = true
    uploadIsError.value = false
    uploadStatus.value = `正在上传 ${file.name || '图片'}…`

    const extMatch = file.type.match(/image\/(png|jpeg|jpg|gif|webp|svg\+xml)/)
    let ext = 'png'
    if (extMatch && extMatch[1]) {
      ext = extMatch[1] === 'jpeg' ? 'jpg' : extMatch[1] === 'svg+xml' ? 'svg' : extMatch[1]
    } else if (file.name.includes('.')) {
      ext = file.name.split('.').pop() || 'png'
    }

    const pad = (n: number) => String(n).padStart(2, '0')
    const now = new Date()
    const timeStr = `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}-${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`
    const randomSuffix = Math.random().toString(36).slice(2, 6)
    const filename = `image-${timeStr}-${randomSuffix}.${ext}`

    const currentPath = props.currentPath || ''
    const parentDir = currentPath.includes('/')
      ? currentPath.substring(0, currentPath.lastIndexOf('/'))
      : ''
    const assetsDir = parentDir ? `${parentDir}/assets` : 'assets'

    await createDir(assetsDir).catch(() => {})

    const uploadPath = `${assetsDir}/${filename}`
    const buffer = await file.arrayBuffer()
    await uploadFile(uploadPath, buffer)

    const relativeRef = `assets/${filename}`
    insertTemplate(`![${filename}](${relativeRef})\n`)

    uploadStatus.value = `✅ 已成功插入图片 ${filename}`
    setTimeout(() => {
      if (uploadStatus.value.includes('已成功插入')) {
        uploadStatus.value = ''
      }
    }, 3000)
  } catch (err) {
    uploadIsError.value = true
    uploadStatus.value = err instanceof Error ? `图片上传失败: ${err.message}` : '图片上传失败'
    setTimeout(() => {
      uploadStatus.value = ''
    }, 4000)
  } finally {
    uploadingImage.value = false
  }
}

async function handlePaste(event: ClipboardEvent): Promise<void> {
  const items = event.clipboardData?.items
  if (!items || items.length === 0) return

  for (const item of items) {
    if (item.type.startsWith('image/')) {
      const file = item.getAsFile()
      if (file) {
        event.preventDefault()
        await uploadAndInsertImage(file)
        return
      }
    }
  }
}

async function handleDrop(event: DragEvent): Promise<void> {
  const files = event.dataTransfer?.files
  if (!files || files.length === 0) return

  const imageFiles = Array.from(files).filter((f) => f.type.startsWith('image/'))
  if (imageFiles.length > 0) {
    event.preventDefault()
    for (const file of imageFiles) {
      await uploadAndInsertImage(file)
    }
  }
}

function triggerImageSelect(): void {
  fileInputRef.value?.click()
}

async function handleFileSelect(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  if (!input.files || input.files.length === 0) return
  const files = Array.from(input.files)
  for (const file of files) {
    await uploadAndInsertImage(file)
  }
  input.value = ''
}

// ---------------- Keyboard Shortcuts ----------------

function handleKeydown(event: KeyboardEvent): void {
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
    event.preventDefault()
    emit('save')
    return
  }

  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'z') {
    event.preventDefault()
    if (event.shiftKey) {
      redo()
    } else {
      undo()
    }
    return
  }

  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'y') {
    event.preventDefault()
    redo()
    return
  }

  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'b') {
    event.preventDefault()
    handleBold()
    return
  }

  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'i') {
    event.preventDefault()
    handleItalic()
    return
  }

  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    handleLink()
    return
  }

  if (event.key === 'Tab') {
    event.preventDefault()
    const textarea = textareaRef.value
    if (!textarea) return
    recordBeforeChange()
    const start = textarea.selectionStart
    const end = textarea.selectionEnd
    const savedScrollTop = textarea.scrollTop
    const savedScrollLeft = textarea.scrollLeft
    const val = localContent.value

    if (start === end) {
      localContent.value = val.substring(0, start) + '  ' + val.substring(end)
      emit('update:content', localContent.value)
      void nextTick(() => {
        textarea.focus({ preventScroll: true })
        textarea.setSelectionRange(start + 2, start + 2)
        textarea.scrollTop = savedScrollTop
        textarea.scrollLeft = savedScrollLeft
        syncScroll()
      })
    } else {
      prefixLines('  ')
    }
  }
}

defineExpose({ getTextareaElement, syncScroll, undo, redo })
</script>

<template>
  <div class="markdown-editor-wrapper">
    <!-- Markdown Formatting Toolbar -->
    <div class="markdown-editor-toolbar" role="toolbar" aria-label="Markdown 编辑工具栏">
      <div class="toolbar-group">
        <button
          class="toolbar-btn"
          type="button"
          title="撤销 (Ctrl+Z)"
          aria-label="撤销"
          :disabled="!canUndo"
          v-html="iconSvg('undo', 14)"
          @mousedown.prevent
          @click="undo"
        ></button>
        <button
          class="toolbar-btn"
          type="button"
          title="重做 (Ctrl+Y / Ctrl+Shift+Z)"
          aria-label="重做"
          :disabled="!canRedo"
          v-html="iconSvg('redo', 14)"
          @mousedown.prevent
          @click="redo"
        ></button>
      </div>

      <div class="toolbar-divider"></div>

      <div class="toolbar-group">
        <button
          class="toolbar-btn"
          type="button"
          title="粗体 (Ctrl+B)"
          aria-label="粗体"
          v-html="iconSvg('bold', 14)"
          @mousedown.prevent
          @click="handleBold"
        ></button>
        <button
          class="toolbar-btn"
          type="button"
          title="斜体 (Ctrl+I)"
          aria-label="斜体"
          v-html="iconSvg('italic', 14)"
          @mousedown.prevent
          @click="handleItalic"
        ></button>
        <button
          class="toolbar-btn"
          type="button"
          title="删除线"
          aria-label="删除线"
          v-html="iconSvg('strikethrough', 14)"
          @mousedown.prevent
          @click="handleStrikethrough"
        ></button>
        <button
          class="toolbar-btn"
          type="button"
          title="待办任务列表 (- [ ] )"
          aria-label="待办任务列表"
          v-html="iconSvg('check-square', 14)"
          @mousedown.prevent
          @click="handleTaskList"
        ></button>
      </div>

      <div class="toolbar-divider"></div>

      <div class="toolbar-group">
        <button
          class="toolbar-btn"
          type="button"
          title="行内代码 (`code`)"
          aria-label="行内代码"
          v-html="iconSvg('code', 14)"
          @mousedown.prevent
          @click="handleInlineCode"
        ></button>
        <button
          class="toolbar-btn"
          type="button"
          title="代码块 (```)"
          aria-label="代码块"
          v-html="iconSvg('file-code', 14)"
          @mousedown.prevent
          @click="handleCodeBlock"
        ></button>
        <button
          class="toolbar-btn"
          type="button"
          title="超链接 (Ctrl+K)"
          aria-label="超链接"
          v-html="iconSvg('link', 14)"
          @mousedown.prevent
          @click="handleLink"
        ></button>
        <button
          class="toolbar-btn"
          type="button"
          title="双向知识链接 ([[文档名]])"
          aria-label="双向知识链接"
          @mousedown.prevent
          @click="handleWikiLink"
        >
          [[]]
        </button>
      </div>

      <div class="toolbar-divider"></div>

      <div class="toolbar-group">
        <button
          class="toolbar-btn"
          type="button"
          title="插入表格"
          aria-label="插入表格"
          v-html="iconSvg('table', 14)"
          @mousedown.prevent
          @click="handleTable"
        ></button>
        <button
          class="toolbar-btn"
          type="button"
          title="上传并插入图片 (也可直接粘贴/拖拽截图)"
          aria-label="上传图片"
          v-html="iconSvg('image', 14)"
          @mousedown.prevent
          @click="triggerImageSelect"
        ></button>
      </div>

      <!-- Hidden file input for image uploads -->
      <input
        ref="fileInputRef"
        type="file"
        accept="image/*"
        multiple
        style="display: none"
        @change="handleFileSelect"
      />
    </div>

    <!-- Upload Status Notification Banner -->
    <div
      v-if="uploadStatus"
      class="editor-upload-status"
      :class="{ error: uploadIsError, active: uploadingImage }"
      role="status"
    >
      <span>{{ uploadStatus }}</span>
    </div>

    <!-- Editor Main Area with Line Numbers -->
    <div class="markdown-editor scroll-surface">
      <pre ref="gutterRef" class="editor-gutter" aria-hidden="true">{{ lineNumbersText }}</pre>
      <div class="editor-workspace">
        <textarea
          ref="textareaRef"
          class="editor-textarea"
          :value="localContent"
          spellcheck="false"
          placeholder="在此输入 Markdown 内容，支持快捷键、双向链接 [[]]，或直接粘贴/拖拽图片…"
          @input="handleInput"
          @scroll="syncScroll($event)"
          @keydown="handleKeydown"
          @paste="handlePaste"
          @dragover.prevent
          @drop="handleDrop"
        ></textarea>
      </div>
    </div>
  </div>
</template>

<style scoped>
.markdown-editor-wrapper {
  display: flex;
  flex-direction: column;
  position: relative;
  width: 100%;
  height: 100%;
  min-height: 400px;
  border: 1px solid var(--border);
  border-radius: 9px;
  background: var(--surface-muted);
  overflow: hidden;
}

.markdown-editor-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: nowrap;
  gap: 3px;
  padding: 4px 8px;
  background: var(--surface);
  border-bottom: 1px solid var(--border);
  user-select: none;
  min-height: 33px;
  z-index: 10;
  overflow-x: auto;
  overflow-y: hidden;
  -webkit-overflow-scrolling: touch;
  scrollbar-width: none;
}

.markdown-editor-toolbar::-webkit-scrollbar {
  display: none;
}

.toolbar-group {
  display: flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
}

.toolbar-divider {
  width: 1px;
  height: 16px;
  background: var(--border);
  margin: 0 3px;
  flex-shrink: 0;
}

.toolbar-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 27px;
  height: 25px;
  padding: 0 5px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--text-muted);
  font-family: var(--font-mono);
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  flex-shrink: 0;
  transition:
    background-color 120ms ease,
    color 120ms ease;
}

.toolbar-btn:hover {
  background: var(--surface-hover);
  color: var(--text);
}

.toolbar-btn:active {
  background: var(--surface-muted);
  transform: translateY(1px);
}

.toolbar-btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
  pointer-events: none;
}

@media (max-width: 640px) {
  .markdown-editor-toolbar {
    padding: 3px 6px;
    gap: 2px;
    min-height: 31px;
  }

  .toolbar-group {
    gap: 1px;
  }

  .toolbar-btn {
    min-width: 25px;
    height: 24px;
    padding: 0 4px;
    font-size: 10.5px;
  }

  .toolbar-divider {
    height: 14px;
    margin: 0 2px;
  }
}

.editor-upload-status {
  display: flex;
  align-items: center;
  padding: 4px 12px;
  background: color-mix(in srgb, var(--accent) 15%, var(--surface));
  color: var(--accent-strong);
  font-size: 12px;
  font-weight: 500;
  border-bottom: 1px solid var(--border);
  animation: fadeIn 150ms ease;
}

.editor-upload-status.error {
  background: color-mix(in srgb, var(--danger, #ef4444) 15%, var(--surface));
  color: var(--danger, #ef4444);
}

@keyframes fadeIn {
  from {
    opacity: 0;
    transform: translateY(-4px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.markdown-editor {
  display: flex;
  position: relative;
  flex: 1;
  width: 100%;
  height: 100%;
  overflow: hidden;
}

.editor-gutter {
  display: block;
  flex-shrink: 0;
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

.editor-textarea {
  width: 100%;
  height: 100%;
  margin: 0;
  padding: 0.9em 1.15em;
  border: none;
  outline: none;
  box-sizing: border-box;
  font-family: var(--font-mono);
  font-size: 13px;
  line-height: 1.45;
  tab-size: 2;
  white-space: pre-wrap;
  word-break: break-word;
  overflow-wrap: break-word;
  overflow: auto;
  color: var(--text);
  background: transparent;
  caret-color: var(--accent-strong, var(--accent));
  resize: none;
}

.editor-textarea::selection {
  background: color-mix(in srgb, var(--accent) 30%, transparent);
}
</style>
