import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import MarkdownViewer from '@/components/MarkdownViewer.vue'

vi.mock('@/api/client', () => ({
  saveTextFile: vi.fn().mockResolvedValue({ item: {} }),
  rawFileUrl: (path: string) => `/api/fs/raw?path=${encodeURIComponent(path)}`,
}))

describe('MarkdownViewer draft auto-save and recovery', () => {
  beforeEach(() => {
    window.localStorage.clear()
  })

  it('detects uncommitted draft from localStorage and displays recovery banner', async () => {
    window.localStorage.setItem(
      'web-reader-draft:notes/todo.md',
      JSON.stringify({
        content: '# Unsaved Draft Content',
        timestamp: Date.now() - 10000,
      }),
    )

    const wrapper = mount(MarkdownViewer, {
      props: {
        content: '# Original Saved Content',
        currentPath: 'notes/todo.md',
        theme: 'night',
      },
    })
    await flushPromises()

    const banner = wrapper.find('.draft-recovery-banner')
    expect(banner.exists()).toBe(true)
    expect(banner.text()).toContain('检测到此文档存在未保存的本地草稿')

    // Click restore
    const restoreBtn = banner.find('.draft-btn.primary')
    await restoreBtn.trigger('click')
    await flushPromises()

    expect(wrapper.find('.draft-recovery-banner').exists()).toBe(false)
  })

  it('discards draft when user clicks discard button', async () => {
    window.localStorage.setItem(
      'web-reader-draft:notes/todo.md',
      JSON.stringify({
        content: '# Unsaved Draft Content',
        timestamp: Date.now() - 10000,
      }),
    )

    const wrapper = mount(MarkdownViewer, {
      props: {
        content: '# Original Saved Content',
        currentPath: 'notes/todo.md',
        theme: 'night',
      },
    })
    await flushPromises()

    const discardBtn = wrapper.findAll('.draft-btn')[1]
    expect(discardBtn?.text()).toBe('放弃草稿')
    await discardBtn?.trigger('click')
    await flushPromises()

    expect(window.localStorage.getItem('web-reader-draft:notes/todo.md')).toBeNull()
    expect(wrapper.find('.draft-recovery-banner').exists()).toBe(false)
  })
})
