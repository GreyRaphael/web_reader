import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import MarkdownEditor from '@/components/MarkdownEditor.vue'

describe('MarkdownEditor component', () => {
  it('renders line numbers based on content line count', async () => {
    const wrapper = mount(MarkdownEditor, {
      props: { content: 'line 1\nline 2\nline 3' },
    })

    const gutter = wrapper.find('.editor-gutter')
    expect(gutter.text()).toBe('1\n2\n3')
  })

  it('emits update:content on textarea input', async () => {
    const wrapper = mount(MarkdownEditor, {
      props: { content: '# Title' },
    })

    const textarea = wrapper.find('textarea')
    await textarea.setValue('# Updated Title')

    expect(wrapper.emitted('update:content')).toBeTruthy()
    expect(wrapper.emitted('update:content')![0]).toEqual(['# Updated Title'])
  })

  it('emits save event on Ctrl+S or Cmd+S keydown', async () => {
    const wrapper = mount(MarkdownEditor, {
      props: { content: 'hello' },
    })

    const textarea = wrapper.find('textarea')
    await textarea.trigger('keydown', { key: 's', ctrlKey: true })

    expect(wrapper.emitted('save')).toBeTruthy()
  })

  it('handles Tab key by inserting spaces', async () => {
    const wrapper = mount(MarkdownEditor, {
      props: { content: 'test' },
    })

    const textarea = wrapper.find('textarea')
    const el = textarea.element as HTMLTextAreaElement
    el.selectionStart = 0
    el.selectionEnd = 0

    await textarea.trigger('keydown', { key: 'Tab' })

    expect(wrapper.emitted('update:content')).toBeTruthy()
    const lastUpdate = wrapper.emitted('update:content')!.at(-1)
    expect(lastUpdate![0]).toBe('  test')
  })
})
