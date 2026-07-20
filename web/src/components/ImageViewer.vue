<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { FsItem } from '@/api/types'
import { AUTH_EXPIRED_EVENT, getSession, rawFileUrl } from '@/api/client'

const props = defineProps<{ item: FsItem }>()
const failed = ref(false)
const loading = ref(true)
const retryKey = ref(0)
const imageSource = computed(() => `${rawFileUrl(props.item.path)}&retry=${retryKey.value}`)

watch(
  () => props.item.path,
  () => {
    failed.value = false
    loading.value = true
    retryKey.value = 0
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
      @load="loading = false"
      @error="handleError"
    />
  </div>
</template>
