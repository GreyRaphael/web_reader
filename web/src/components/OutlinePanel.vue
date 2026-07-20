<script setup lang="ts">
import { computed } from 'vue'
import type { MarkdownHeading } from '@/markdown/render'

const props = defineProps<{ headings: MarkdownHeading[]; activeId: string }>()
const minimumLevel = computed(() =>
  props.headings.length > 0 ? Math.min(...props.headings.map((heading) => heading.level)) : 1,
)
const emit = defineEmits<{ select: [id: string] }>()
</script>

<template>
  <div class="outline-panel">
    <div class="panel-heading">
      <div>
        <p class="panel-kicker">ON THIS PAGE</p>
        <h2>大纲</h2>
      </div>
    </div>
    <nav v-if="headings.length" class="outline-nav" aria-label="文章大纲">
      <button
        v-for="heading in headings"
        :key="heading.id"
        class="outline-link"
        :class="{ active: heading.id === activeId }"
        :style="{ '--outline-depth': Math.max(0, heading.level - minimumLevel) }"
        :aria-current="heading.id === activeId ? 'location' : undefined"
        type="button"
        @click="emit('select', heading.id)"
      >
        {{ heading.title }}
      </button>
    </nav>
    <div v-else class="outline-empty">
      <span aria-hidden="true">☷</span>
      <p>打开 Markdown 文件后，这里会显示章节大纲。</p>
    </div>
  </div>
</template>
