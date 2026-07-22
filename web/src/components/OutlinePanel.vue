<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { MarkdownHeading } from '@/markdown/render'
import { iconSvg } from '@/utils/icons'

const props = defineProps<{ headings: MarkdownHeading[]; activeId: string }>()
const emit = defineEmits<{ select: [id: string]; close: [] }>()

const minimumLevel = computed(() =>
  props.headings.length > 0 ? Math.min(...props.headings.map((heading) => heading.level)) : 1,
)

const collapsedIds = ref(new Set<string>())

const headingsWithMeta = computed(() => {
  return props.headings.map((h, i) => {
    const next = props.headings[i + 1]
    const hasChild = next ? next.level > h.level : false
    return { ...h, originalIndex: i, hasChild }
  })
})

const visibleHeadings = computed(() => {
  const result = []
  let activeCollapseLevel = -1
  for (const h of headingsWithMeta.value) {
    if (activeCollapseLevel !== -1) {
      if (h.level > activeCollapseLevel) {
        continue
      } else {
        activeCollapseLevel = -1
      }
    }
    result.push(h)
    if (collapsedIds.value.has(h.id)) {
      activeCollapseLevel = h.level
    }
  }
  return result
})

function toggleCollapse(id: string, event: Event) {
  event.stopPropagation()
  if (collapsedIds.value.has(id)) {
    collapsedIds.value.delete(id)
  } else {
    collapsedIds.value.add(id)
  }
}


function collapseLevel2() {
  const newSet = new Set<string>()
  headingsWithMeta.value.forEach((h) => {
    if (h.hasChild && h.level >= minimumLevel.value + 1) {
      newSet.add(h.id)
    }
  })
  collapsedIds.value = newSet
}

function expandAll() {
  collapsedIds.value.clear()
}

watch(
  () => props.headings,
  () => {
    collapsedIds.value.clear()
  }
)
</script>

<template>
  <div class="outline-panel">
    <div class="panel-heading">
      <div>
        <h2>OUTLINE</h2>
      </div>
      <div class="tree-toolbar">
        <button
          class="icon-button compact"
          type="button"
          title="折叠 Level 2"
          aria-label="折叠 Level 2"
          v-html="iconSvg('chevrons-up', 16)"
          @click="collapseLevel2"
        ></button>
        <button
          class="icon-button compact"
          type="button"
          title="展开所有"
          aria-label="展开所有"
          v-html="iconSvg('chevrons-down', 16)"
          @click="expandAll"
        ></button>
        <button
          class="icon-button compact drawer-close-btn"
          type="button"
          title="关闭大纲栏"
          aria-label="关闭大纲栏"
          @click="emit('close')"
        >
          ×
        </button>
      </div>
    </div>
    <nav v-if="headings.length" class="outline-nav" aria-label="文章大纲">
      <button
        v-for="heading in visibleHeadings"
        :key="heading.id"
        class="outline-link"
        :class="{ active: heading.id === activeId }"
        :style="{ '--outline-depth': Math.max(0, heading.level - minimumLevel) }"
        :aria-current="heading.id === activeId ? 'location' : undefined"
        type="button"
        @click="emit('select', heading.id)"
      >
        <span
          v-if="heading.hasChild"
          class="outline-chevron"
          v-html="iconSvg(collapsedIds.has(heading.id) ? 'chevron-right' : 'chevron-down', 14)"
          @click.stop="toggleCollapse(heading.id, $event)"
        ></span>
        <span v-else class="outline-chevron-spacer"></span>
        <span class="outline-title">{{ heading.title }}</span>
        <span
          v-if="heading.level > minimumLevel"
          class="outline-guide"
          :style="{ left: `${10 + (heading.level - minimumLevel - 1) * 13}px` }"
        ></span>
      </button>
    </nav>
    <div v-else class="outline-empty">
      <span aria-hidden="true" v-html="iconSvg('list', 28)"></span>
      <p>打开 Markdown 文件后，这里会显示章节大纲。</p>
    </div>
  </div>
</template>
