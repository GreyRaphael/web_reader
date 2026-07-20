<script setup lang="ts">
import { onErrorCaptured, ref, watch } from 'vue'

const props = defineProps<{ resetKey?: string }>()
const errorMessage = ref('')

onErrorCaptured((error) => {
  errorMessage.value = error instanceof Error ? error.message : '界面渲染失败'
  return false
})

watch(
  () => props.resetKey,
  () => {
    errorMessage.value = ''
  },
)
</script>

<template>
  <slot v-if="!errorMessage" />
  <div v-else class="error-boundary" role="alert">
    <div class="state-icon" aria-hidden="true">!</div>
    <h2>此区域未能正常显示</h2>
    <p>{{ errorMessage }}</p>
    <button class="primary-button" type="button" @click="errorMessage = ''">重试</button>
  </div>
</template>
