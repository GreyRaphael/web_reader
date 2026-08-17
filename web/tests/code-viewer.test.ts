import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import CodeViewer from '@/components/CodeViewer.vue'

describe('CodeViewer', () => {
  beforeEach(() => {
    window.localStorage.clear()
    const writeTextMock = vi.fn().mockResolvedValue(undefined)
    Object.assign(navigator, {
      clipboard: { writeText: writeTextMock },
    })
  })

  it('renders content, line count, language badge, and action buttons', async () => {
    const code = 'const a = 1\nconst b = 2\nconst c = 3'
    const wrapper = mount(CodeViewer, {
      props: {
        content: code,
        path: 'src/index.ts',
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('3 行')
    expect(wrapper.text()).toContain('TYPESCRIPT')
    expect(wrapper.find('.code-gutter').text()).toContain('1\n2\n3')
    expect(wrapper.find('.code-tool-btn').text()).toContain('换行')
  })

  it('toggles word wrap on button click and updates localStorage and classes', async () => {
    const code = 'const longLine = "very long string content here"'
    const wrapper = mount(CodeViewer, {
      props: {
        content: code,
        path: 'src/index.ts',
      },
    })
    await flushPromises()

    const body = wrapper.find('.code-viewer-body')
    expect(body.classes()).not.toContain('word-wrap')

    const wrapBtn = wrapper.findAll('.code-tool-btn')[0]
    expect(wrapBtn).toBeDefined()
    await wrapBtn!.trigger('click')
    await flushPromises()

    expect(body.classes()).toContain('word-wrap')
    expect(wrapBtn!.text()).toContain('已换行')
    expect(window.localStorage.getItem('web-reader-code-wrap')).toBe('true')

    await wrapBtn!.trigger('click')
    await flushPromises()

    expect(body.classes()).not.toContain('word-wrap')
    expect(wrapBtn!.text()).toContain('换行')
    expect(window.localStorage.getItem('web-reader-code-wrap')).toBe('false')
  })

  it('restores word wrap preference from localStorage on mount', async () => {
    window.localStorage.setItem('web-reader-code-wrap', 'true')
    const wrapper = mount(CodeViewer, {
      props: {
        content: 'hello world',
        path: 'notes.txt',
      },
    })
    await flushPromises()

    const body = wrapper.find('.code-viewer-body')
    expect(body.classes()).toContain('word-wrap')
    const wrapBtn = wrapper.findAll('.code-tool-btn')[0]
    expect(wrapBtn!.text()).toContain('已换行')
  })

  it('copies code to clipboard', async () => {
    const code = 'console.log("Antigravity")'
    const wrapper = mount(CodeViewer, {
      props: {
        content: code,
        path: 'main.js',
      },
    })
    await flushPromises()

    const copyBtn = wrapper.find('.code-copy-btn')
    await copyBtn.trigger('click')
    await flushPromises()

    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(code)
    expect(copyBtn.text()).toContain('已复制')
  })
})
