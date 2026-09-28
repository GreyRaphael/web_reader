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

describe('MarkdownViewer export to PDF', () => {
  beforeEach(() => {
    window.localStorage.clear()
    window.print = vi.fn()
  })

  it('renders export PDF button to the right of save button', async () => {
    const wrapper = mount(MarkdownViewer, {
      props: {
        content: '# Test Document\n\nSome text.',
        currentPath: 'notes/doc.md',
        theme: 'night',
      },
    })
    await flushPromises()

    const toolbarActions = wrapper.find('.markdown-toolbar-actions')
    expect(toolbarActions.exists()).toBe(true)

    const buttons = toolbarActions.findAll('button')
    // Save button is first button, Export PDF button is immediately after it
    const saveBtn = toolbarActions.find('.save-btn')
    const exportBtn = toolbarActions.find('.export-pdf-btn')

    expect(saveBtn.exists()).toBe(true)
    expect(exportBtn.exists()).toBe(true)
    expect(exportBtn.text()).toContain('导出 PDF')

    // Verify ordering in DOM: export button comes after save button
    const saveIdx = buttons.findIndex((b) => b.classes().includes('save-btn'))
    const exportIdx = buttons.findIndex((b) => b.classes().includes('export-pdf-btn'))
    expect(exportIdx).toBe(saveIdx + 1)
  })

  it('triggers window.print when export PDF button is clicked', async () => {
    const printSpy = vi.fn()
    window.print = printSpy

    const wrapper = mount(MarkdownViewer, {
      props: {
        content: '# Test Document\n\nContent for print.',
        currentPath: 'notes/doc.md',
        theme: 'night',
      },
    })
    await flushPromises()

    const exportBtn = wrapper.find('.export-pdf-btn')
    await exportBtn.trigger('click')
    await flushPromises()

    expect(printSpy).toHaveBeenCalledTimes(1)
  })

  it('switches to preview before printing when in edit mode', async () => {
    window.localStorage.setItem('web-reader-markdown-view-mode', 'edit')
    const printSpy = vi.fn()
    window.print = printSpy

    const wrapper = mount(MarkdownViewer, {
      props: {
        content: '# Test Document\n\nContent for print.',
        currentPath: 'notes/doc.md',
        theme: 'night',
      },
    })
    await flushPromises()

    // Initially in edit mode, article should not be rendered
    expect(wrapper.find('.editor-pane').exists()).toBe(true)
    expect(wrapper.find('article.markdown-body').exists()).toBe(false)

    const exportBtn = wrapper.find('.export-pdf-btn')
    await exportBtn.trigger('click')
    await flushPromises()

    // print should be called and article should have rendered
    expect(printSpy).toHaveBeenCalledTimes(1)
    expect(wrapper.find('article.markdown-body').exists()).toBe(true)
  })
})

