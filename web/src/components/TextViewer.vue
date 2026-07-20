<script setup lang="ts">
defineProps<{ content: string; encoding: string }>()
</script>

<template>
  <article class="text-viewer" aria-label="文本内容">
    <div v-if="encoding" class="text-encoding">{{ encoding }}</div>
    <pre><code>{{ content }}</code></pre>
  </article>
</template>
