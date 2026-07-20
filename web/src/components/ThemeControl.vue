<script setup lang="ts">
import { computed } from 'vue'
import { useTheme, type ThemeMode } from '@/composables/useTheme'

const { mode, setMode } = useTheme()

const options: Array<{ value: ThemeMode; label: string; icon: string }> = [
  { value: 'day', label: '日间主题', icon: '☀' },
  { value: 'night', label: '夜间主题', icon: '☾' },
  { value: 'system', label: '跟随系统', icon: '◐' },
]

const current = computed(() => options.find((option) => option.value === mode.value) ?? options[2]!)

function cycleTheme(): void {
  const index = options.findIndex((option) => option.value === mode.value)
  setMode(options[(index + 1) % options.length]!.value)
}
</script>

<template>
  <div class="theme-switcher">
    <div class="theme-control" role="group" aria-label="主题">
      <button
        v-for="option in options"
        :key="option.value"
        class="theme-option"
        :class="{ active: mode === option.value }"
        type="button"
        :aria-label="option.label"
        :aria-pressed="mode === option.value"
        :title="option.label"
        @click="setMode(option.value)"
      >
        <span aria-hidden="true">{{ option.icon }}</span>
      </button>
    </div>
    <button
      class="theme-mobile-toggle icon-button"
      type="button"
      :aria-label="`${current.label}，点击切换主题`"
      :title="`${current.label}，点击切换`"
      @click="cycleTheme"
    >
      <span aria-hidden="true">{{ current.icon }}</span>
    </button>
  </div>
</template>
