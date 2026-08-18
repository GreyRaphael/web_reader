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

  it('formats text when toolbar buttons are clicked', async () => {
    const wrapper = mount(MarkdownEditor, {
      props: { content: 'Hello' },
    })

    const textarea = wrapper.find('textarea')
    const el = textarea.element as HTMLTextAreaElement
    el.selectionStart = 0
    el.selectionEnd = 5

    // Click Bold
    const boldBtn = wrapper.find('button[aria-label="粗体"]')
    expect(boldBtn.exists()).toBe(true)
    await boldBtn.trigger('click')

    expect(wrapper.emitted('update:content')).toBeTruthy()
    let lastUpdate = wrapper.emitted('update:content')!.at(-1)
    expect(lastUpdate![0]).toBe('**Hello**')

    // Click Task List
    const taskBtn = wrapper.find('button[aria-label="待办任务列表"]')
    expect(taskBtn.exists()).toBe(true)
    await taskBtn.trigger('click')

    lastUpdate = wrapper.emitted('update:content')!.at(-1)
    expect(lastUpdate![0]).toBe('- [ ] **Hello**')
  })

  it('handles Ctrl+B and Ctrl+I shortcuts for bold and italic formatting', async () => {
    const wrapper = mount(MarkdownEditor, {
      props: { content: 'text' },
    })

    const textarea = wrapper.find('textarea')
    const el = textarea.element as HTMLTextAreaElement
    el.selectionStart = 0
    el.selectionEnd = 4

    await textarea.trigger('keydown', { key: 'b', ctrlKey: true })

    let lastUpdate = wrapper.emitted('update:content')!.at(-1)
    expect(lastUpdate![0]).toBe('**text**')

    await textarea.trigger('keydown', { key: 'i', ctrlKey: true })
    lastUpdate = wrapper.emitted('update:content')!.at(-1)
    expect(lastUpdate![0]).toBe('***text***')
  })

  it('cancels/unwraps bold, italic, and strikethrough when already formatted', async () => {
    const wrapper = mount(MarkdownEditor, {
      props: { content: '**already bold** and *italic* and ~~striked~~' },
    })

    const textarea = wrapper.find('textarea')
    const el = textarea.element as HTMLTextAreaElement

    // 1. Select "**already bold**" and click Bold -> unwrap to "already bold"
    el.selectionStart = 0
    el.selectionEnd = 16
    const boldBtn = wrapper.find('button[aria-label="粗体"]')
    await boldBtn.trigger('click')

    let lastUpdate = wrapper.emitted('update:content')!.at(-1)
    expect(lastUpdate![0]).toBe('already bold and *italic* and ~~striked~~')

    // 2. Select "*italic*" (now at index 17 to 25) and click Italic -> unwrap to "italic"
    await wrapper.setProps({ content: lastUpdate![0] as string })
    el.selectionStart = 17
    el.selectionEnd = 25
    const italicBtn = wrapper.find('button[aria-label="斜体"]')
    await italicBtn.trigger('click')

    lastUpdate = wrapper.emitted('update:content')!.at(-1)
    expect(lastUpdate![0]).toBe('already bold and italic and ~~striked~~')

    // 3. Select "~~striked~~" (now at index 28 to 39) and click Strikethrough -> unwrap to "striked"
    await wrapper.setProps({ content: lastUpdate![0] as string })
    el.selectionStart = 28
    el.selectionEnd = 39
    const strikeBtn = wrapper.find('button[aria-label="删除线"]')
    await strikeBtn.trigger('click')

    lastUpdate = wrapper.emitted('update:content')!.at(-1)
    expect(lastUpdate![0]).toBe('already bold and italic and striked')
  })

  it('toggles task list prefix on and off', async () => {
    const wrapper = mount(MarkdownEditor, {
      props: { content: '- [ ] Item 1\n- [ ] Item 2' },
    })

    const textarea = wrapper.find('textarea')
    const el = textarea.element as HTMLTextAreaElement
    el.selectionStart = 0
    el.selectionEnd = 25

    // Click Task List on existing task list -> toggles off to "Item 1\nItem 2"
    const taskBtn = wrapper.find('button[aria-label="待办任务列表"]')
    await taskBtn.trigger('click')

    const lastUpdate = wrapper.emitted('update:content')!.at(-1)
    expect(lastUpdate![0]).toBe('Item 1\nItem 2')
  })

  it('supports Undo (Ctrl+Z) and Redo (Ctrl+Y / Ctrl+Shift+Z)', async () => {
    const wrapper = mount(MarkdownEditor, {
      props: { content: 'Initial text' },
    })

    const textarea = wrapper.find('textarea')
    const el = textarea.element as HTMLTextAreaElement

    // Format text with bold
    el.selectionStart = 0
    el.selectionEnd = 7
    const boldBtn = wrapper.find('button[aria-label="粗体"]')
    await boldBtn.trigger('click')

    let lastUpdate = wrapper.emitted('update:content')!.at(-1)
    expect(lastUpdate![0]).toBe('**Initial** text')

    // Press Ctrl+Z to Undo
    await textarea.trigger('keydown', { key: 'z', ctrlKey: true })
    lastUpdate = wrapper.emitted('update:content')!.at(-1)
    expect(lastUpdate![0]).toBe('Initial text')

    // Press Ctrl+Y to Redo
    await textarea.trigger('keydown', { key: 'y', ctrlKey: true })
    lastUpdate = wrapper.emitted('update:content')!.at(-1)
    expect(lastUpdate![0]).toBe('**Initial** text')

    // Press Ctrl+Shift+Z to Undo and Redo
    await textarea.trigger('keydown', { key: 'z', ctrlKey: true })
    lastUpdate = wrapper.emitted('update:content')!.at(-1)
    expect(lastUpdate![0]).toBe('Initial text')

    await textarea.trigger('keydown', { key: 'z', ctrlKey: true, shiftKey: true })
    lastUpdate = wrapper.emitted('update:content')!.at(-1)
    expect(lastUpdate![0]).toBe('**Initial** text')

    // Click Toolbar Undo button
    const undoBtn = wrapper.find('button[aria-label="撤销"]')
    expect(undoBtn.attributes('disabled')).toBeUndefined()
    await undoBtn.trigger('click')

    lastUpdate = wrapper.emitted('update:content')!.at(-1)
    expect(lastUpdate![0]).toBe('Initial text')
  })
})
