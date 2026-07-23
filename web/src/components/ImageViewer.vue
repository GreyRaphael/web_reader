<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import type { FsItem } from '@/api/types'
import { AUTH_EXPIRED_EVENT, getSession, rawFileUrl } from '@/api/client'
import { ICON_PATHS } from '@/utils/icons'

const props = defineProps<{ item: FsItem }>()
const failed = ref(false)
const loading = ref(true)
const retryKey = ref(0)
const lightboxOpen = ref(false)
const lightboxRef = ref<HTMLDialogElement | null>(null)
let previouslyFocused: HTMLElement | null = null
const imageSource = computed(() => `${rawFileUrl(props.item.path)}&retry=${retryKey.value}`)

watch(
  () => props.item.path,
  () => {
    failed.value = false
    loading.value = true
    retryKey.value = 0
    closeLightbox()
  },
)

async function handleError(): Promise<void> {
  failed.value = true
  loading.value = false
  try {
    const session = await getSession()
    if (!session.authenticated) window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT))
  } catch {
    // Keep the image-specific error visible when the session check also fails.
  }
}

function retry(): void {
  failed.value = false
  loading.value = true
  retryKey.value += 1
}

function openLightbox(): void {
  previouslyFocused = document.activeElement instanceof HTMLElement ? document.activeElement : null
  lightboxOpen.value = true
  nextTick(() => {
    const dialog = lightboxRef.value
    if (dialog && !dialog.open) dialog.showModal()
    lightboxRef.value?.querySelector<HTMLButtonElement>('.lightbox-close')?.focus()
  })
}

function closeLightbox(): void {
  lightboxOpen.value = false
  previouslyFocused?.focus()
  previouslyFocused = null
}

onBeforeUnmount(() => {
  if (lightboxOpen.value) {
    if (lightboxRef.value?.open) lightboxRef.value.close()
    previouslyFocused?.focus()
    previouslyFocused = null
  }
})
</script>

<template>
  <div class="image-viewer">
    <div v-if="loading && !failed" class="image-loading" role="status">
      <span class="loading-ring" aria-hidden="true"></span>
      <span>加载图片…</span>
    </div>
    <div v-if="failed" class="preview-state error" role="alert">
      <div class="state-icon">!</div>
      <h2>图片加载失败</h2>
      <button class="secondary-button" type="button" @click="retry">重试</button>
    </div>
    <img
      v-show="!failed"
      :src="imageSource"
      :alt="item.name"
      class="image-viewer-img"
      @load="loading = false"
      @error="handleError"
      @click="openLightbox"
    />

    <Teleport to="body">
      <dialog
        v-if="lightboxOpen"
        ref="lightboxRef"
        class="lightbox-backdrop"
        @click.self="closeLightbox"
        @cancel.prevent="closeLightbox"
      >
        <button class="lightbox-close" @click="closeLightbox" aria-label="关闭">
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" v-html="ICON_PATHS['x']"></svg>
        </button>
        <img
          :src="imageSource"
          :alt="item.name"
          class="lightbox-image"
          @click.stop="closeLightbox"
        />
      </dialog>
    </Teleport>
  </div>
</template>
