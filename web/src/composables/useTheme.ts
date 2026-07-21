import { computed, ref, watch, type ComputedRef, type Ref } from 'vue'

export type ThemeMode = 'day' | 'night' | 'system'
export type ResolvedTheme = 'day' | 'night'

const STORAGE_KEY = 'web-reader-theme'
const allowedModes = new Set<ThemeMode>(['day', 'night', 'system'])
const THEME_COLORS: Record<ResolvedTheme, string> = {
  day: '#f7f7f5',
  night: '#111513',
}

function savedMode(): ThemeMode {
  if (typeof window === 'undefined') return 'system'
  try {
    const value = window.localStorage.getItem(STORAGE_KEY) as ThemeMode | null
    return value && allowedModes.has(value) ? value : 'system'
  } catch {
    return 'system'
  }
}

const mode = ref<ThemeMode>(savedMode())
const systemIsNight = ref(false)
let initialized = false

function initialize(): void {
  if (initialized || typeof window === 'undefined') return
  initialized = true
  const media = window.matchMedia('(prefers-color-scheme: dark)')
  systemIsNight.value = media.matches
  media.addEventListener('change', (event) => {
    systemIsNight.value = event.matches
  })
}

const resolved = computed<ResolvedTheme>(() => {
  if (mode.value === 'system') return systemIsNight.value ? 'night' : 'day'
  return mode.value
})

function applyTheme(): void {
  if (typeof document === 'undefined') return
  document.documentElement.dataset.theme = resolved.value
  document.documentElement.style.colorScheme = resolved.value === 'night' ? 'dark' : 'light'
  const meta = document.querySelector<HTMLMetaElement>('meta[name="theme-color"]')
  meta?.setAttribute('content', THEME_COLORS[resolved.value])
}

watch([mode, resolved], () => {
  try {
    window.localStorage.setItem(STORAGE_KEY, mode.value)
  } catch {
    // The selected theme still applies when storage is unavailable.
  }
  applyTheme()
})

export interface ThemeController {
  mode: Ref<ThemeMode>
  resolved: ComputedRef<ResolvedTheme>
  setMode: (value: ThemeMode) => void
}

export function useTheme(): ThemeController {
  initialize()
  applyTheme()
  return {
    mode,
    resolved,
    setMode(value) {
      mode.value = value
    },
  }
}
