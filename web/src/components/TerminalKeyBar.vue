<script setup lang="ts">
import { ref } from 'vue'

const emit = defineEmits<{ key: [data: string] }>()

const ctrlActive = ref(false)

const keys = [
  { label: 'ESC', value: '\x1b' },
  { label: 'TAB', value: '\t' },
  { label: '↑', value: '\x1b[A' },
  { label: '↓', value: '\x1b[B' },
  { label: '←', value: '\x1b[D' },
  { label: '→', value: '\x1b[C' },
  { label: '/', value: '/' },
  { label: '-', value: '-' },
  { label: '|', value: '|' },
  { label: '~', value: '~' },
]

function sendKey(value: string) {
  if (ctrlActive.value) {
    // Prefix with Ctrl for single-char keys.
    const code = value.charCodeAt(0)
    if (value.length === 1 && code >= 0x40 && code <= 0x7e) {
      emit('key', String.fromCharCode(code & 0x1f))
    } else {
      emit('key', value)
    }
    ctrlActive.value = false
  } else {
    emit('key', value)
  }
}

function toggleCtrl() {
  ctrlActive.value = !ctrlActive.value
}
</script>

<template>
  <div class="terminal-keybar" role="toolbar" aria-label="终端辅助键">
    <button
      class="key-btn"
      :class="{ active: ctrlActive }"
      type="button"
      :aria-pressed="ctrlActive"
      title="Ctrl 组合键：点亮后下一次按键发送 Ctrl+键"
      @click="toggleCtrl"
    >
      CTRL
    </button>
    <button
      v-for="k in keys"
      :key="k.label"
      class="key-btn"
      type="button"
      @click="sendKey(k.value)"
    >
      {{ k.label }}
    </button>
  </div>
</template>

<style scoped>
.terminal-keybar {
  display: flex;
  gap: 4px;
  padding: 4px 8px;
  border-bottom: 1px solid var(--border);
  background: var(--surface-raised);
  overflow-x: auto;
  scrollbar-width: none;
}

.terminal-keybar::-webkit-scrollbar {
  display: none;
}

.key-btn {
  flex-shrink: 0;
  min-width: 32px;
  height: 30px;
  padding: 0 8px;
  border: 1px solid var(--border);
  border-radius: 5px;
  background: var(--surface);
  color: var(--text);
  font-family: var(--font-mono);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: background-color 120ms ease;
}

.key-btn:hover {
  background: var(--surface-hover);
}

.key-btn.active {
  background: var(--accent);
  color: #fff;
  border-color: var(--accent);
}
</style>
