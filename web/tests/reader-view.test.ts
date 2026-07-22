import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ReaderView from '@/views/ReaderView.vue'

function installMatchMedia(mobile: boolean): void {
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: query.includes('max-width') ? mobile : false,
    media: query,
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }))
}

const stubs = {
  FileTree: { template: '<div><button aria-label="关闭文件栏">×</button></div>' },
  OutlinePanel: { template: '<div><button aria-label="关闭大纲栏">×</button></div>' },
  PreviewPane: true,
  ThemeControl: true,
}

describe('ReaderView layout', () => {
  beforeEach(() => {
    window.localStorage.clear()
    window.history.replaceState({}, '', '/')
  })

  it('manages mobile drawer focus, background isolation, and Escape', async () => {
    installMatchMedia(true)
    const wrapper = mount(ReaderView, {
      props: { username: 'admin' },
      global: { stubs },
      attachTo: document.body,
    })

    const toggle = wrapper.get('#left-panel-toggle')
    await toggle.trigger('click')

    const drawer = wrapper.get('#left-panel')
    expect(toggle.attributes('aria-expanded')).toBe('true')
    expect(drawer.attributes('role')).toBe('dialog')
    expect(drawer.attributes('aria-modal')).toBe('true')
    expect(drawer.attributes()).not.toHaveProperty('inert')
    expect(wrapper.get('preview-pane-stub').attributes()).toHaveProperty('inert')
    expect(document.activeElement?.getAttribute('aria-label')).toBe('关闭文件栏')

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await wrapper.vm.$nextTick()

    expect(toggle.attributes('aria-expanded')).toBe('false')
    expect(document.activeElement).toBe(toggle.element)
    wrapper.unmount()
  })

  it('persists desktop panel visibility', async () => {
    installMatchMedia(false)
    const wrapper = mount(ReaderView, {
      props: { username: 'admin' },
      global: { stubs },
    })

    await wrapper.get('#left-panel-toggle').trigger('click')
    await wrapper.vm.$nextTick()

    expect(window.localStorage.getItem('web-reader-left-visible')).toBe('false')
    expect(wrapper.get('#left-panel').attributes()).toHaveProperty('inert')
  })
})
