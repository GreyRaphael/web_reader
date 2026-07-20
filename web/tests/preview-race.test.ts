import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h, nextTick } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import type { FsItem, TextResponse } from '@/api/types'
import ReaderView from '@/views/ReaderView.vue'

const getTextFileMock = vi.hoisted(() => vi.fn())
const logoutMock = vi.hoisted(() => vi.fn())

vi.mock('@/api/client', () => ({
  getFileMeta: vi.fn(),
  getTextFile: getTextFileMock,
  logout: logoutMock,
}))

function deferred<T>(): { promise: Promise<T>; resolve: (value: T) => void } {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => {
    resolve = done
  })
  return { promise, resolve }
}

function item(path: string): FsItem {
  return {
    path,
    name: path,
    kind: 'file',
    previewKind: 'markdown',
    size: 10,
    modifiedAt: '2026-07-20T00:00:00Z',
    mime: 'text/markdown',
  }
}

const firstItem = item('first.md')
const secondItem = item('second.md')

const FileTreeStub = defineComponent({
  props: ['selectedPath'],
  emits: ['open'],
  setup(_props, { emit }) {
    return () =>
      h('div', [
        h('button', { id: 'open-first', onClick: () => emit('open', firstItem) }, 'First'),
        h('button', { id: 'open-second', onClick: () => emit('open', secondItem) }, 'Second'),
      ])
  },
})

const PreviewPaneStub = defineComponent({
  props: ['text'],
  template: '<div id="preview-content">{{ text?.content || "" }}</div>',
})

describe('Reader preview races', () => {
  it('does not let a stale text response replace the current file', async () => {
    window.matchMedia = vi.fn().mockReturnValue({
      matches: false,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })
    const first = deferred<TextResponse>()
    const second = deferred<TextResponse>()
    getTextFileMock.mockReset()
    getTextFileMock.mockImplementation((path: string) =>
      path === firstItem.path ? first.promise : second.promise,
    )

    const wrapper = mount(ReaderView, {
      props: { username: 'admin' },
      global: {
        stubs: {
          FileTree: FileTreeStub,
          OutlinePanel: true,
          PreviewPane: PreviewPaneStub,
          ThemeControl: true,
        },
      },
    })

    await wrapper.get('#open-first').trigger('click')
    await wrapper.get('#open-second').trigger('click')
    second.resolve({
      path: secondItem.path,
      content: 'second content',
      encoding: 'utf-8',
      size: 14,
      modifiedAt: secondItem.modifiedAt,
    })
    await flushPromises()
    expect(wrapper.get('#preview-content').text()).toBe('second content')

    first.resolve({
      path: firstItem.path,
      content: 'stale first content',
      encoding: 'utf-8',
      size: 19,
      modifiedAt: firstItem.modifiedAt,
    })
    await flushPromises()
    await nextTick()

    expect(wrapper.get('#preview-content').text()).toBe('second content')
    expect(wrapper.text()).toContain('second.md')
  })
})
