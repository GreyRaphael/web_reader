import { nextTick } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

function installMatchMedia(matches: boolean): void {
  window.matchMedia = vi.fn().mockReturnValue({
    matches,
    media: '(prefers-color-scheme: dark)',
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })
}

describe('useTheme', () => {
  beforeEach(() => {
    vi.resetModules()
    window.localStorage.clear()
    delete document.documentElement.dataset.theme
    installMatchMedia(false)
  })

  it('restores and persists the selected theme', async () => {
    window.localStorage.setItem('web-reader-theme', 'night')
    const { useTheme } = await import('@/composables/useTheme')
    const theme = useTheme()

    expect(theme.mode.value).toBe('night')
    expect(theme.resolved.value).toBe('night')
    expect(document.documentElement.dataset.theme).toBe('night')

    theme.setMode('day')
    await nextTick()

    expect(window.localStorage.getItem('web-reader-theme')).toBe('day')
    expect(document.documentElement.dataset.theme).toBe('day')
  })
})
