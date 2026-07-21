<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { iconSvg } from '@/utils/icons'

export interface ContextMenuItem {
  label: string
  icon?: string
  action: () => void
  danger?: boolean
  separator?: boolean
}

const props = defineProps<{
  items: ContextMenuItem[]
  x: number
  y: number
}>()
const emit = defineEmits<{ close: [] }>()

const menuRef = ref<HTMLElement | null>(null)
const adjustedX = ref(props.x)
const adjustedY = ref(props.y)

function adjustPosition(): void {
  const menu = menuRef.value
  if (!menu) return
  const rect = menu.getBoundingClientRect()
  if (props.x + rect.width > window.innerWidth) adjustedX.value = props.x - rect.width
  if (props.y + rect.height > window.innerHeight) adjustedY.value = props.y - rect.height
}

function handleKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape') {
    event.preventDefault()
    emit('close')
  }
}

function close(handler: () => void): void {
  handler()
  emit('close')
}

watch(
  () => props.items,
  async () => {
    await nextTick()
    adjustPosition()
    const first = menuRef.value?.querySelector<HTMLElement>('.ctx-item')
    first?.focus()
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  document.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <Teleport to="body">
    <div class="ctx-overlay" @click="emit('close')" @contextmenu.prevent="emit('close')"></div>
    <div
      ref="menuRef"
      class="ctx-menu"
      role="menu"
      :style="{ left: adjustedX + 'px', top: adjustedY + 'px' }"
      @keydown="handleKeydown"
    >
      <template v-for="(item, index) in items" :key="index">
        <div v-if="item.separator" class="ctx-separator" role="separator"></div>
        <button
          v-else
          class="ctx-item"
          :class="{ danger: item.danger }"
          type="button"
          role="menuitem"
          @click="close(item.action)"
        >
          <span v-if="item.icon" class="ctx-icon" v-html="iconSvg(item.icon, 14)"></span>
          <span class="ctx-label">{{ item.label }}</span>
        </button>
      </template>
    </div>
  </Teleport>
</template>