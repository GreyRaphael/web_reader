import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import FileTree from '@/components/FileTree.vue'
import type { FsItem } from '@/api/types'

const listDirectoryMock = vi.hoisted(() => vi.fn())

vi.mock('@/api/client', () => ({
  listDirectory: listDirectoryMock,
}))

const directory: FsItem = {
  path: 'book1',
  name: 'book1',
  kind: 'directory',
  previewKind: 'unsupported',
  size: 0,
  modifiedAt: '2026-07-20T00:00:00Z',
  mime: '',
}

const markdown: FsItem = {
  path: 'book1/chapter1.md',
  name: 'chapter1.md',
  kind: 'file',
  previewKind: 'markdown',
  size: 128,
  modifiedAt: '2026-07-20T00:00:00Z',
  mime: 'text/markdown',
}

describe('FileTree', () => {
  beforeEach(() => {
    listDirectoryMock.mockReset()
    listDirectoryMock.mockImplementation(async (path: string) => ({
      items: path === '' ? [directory] : [markdown],
    }))
  })

  it('loads directories lazily and refreshes expanded nodes', async () => {
    const wrapper = mount(FileTree, { props: { selectedPath: '' } })
    await flushPromises()

    expect(listDirectoryMock).toHaveBeenCalledTimes(1)
    expect(listDirectoryMock).toHaveBeenLastCalledWith('', expect.any(AbortSignal))

    await wrapper.get('.tree-row').trigger('click')
    await flushPromises()
    expect(listDirectoryMock).toHaveBeenCalledTimes(2)
    expect(listDirectoryMock).toHaveBeenLastCalledWith('book1', expect.any(AbortSignal))
    expect(wrapper.text()).toContain('chapter1.md')

    await wrapper.get('button[aria-label="刷新文件树"]').trigger('click')
    await flushPromises()

    expect(listDirectoryMock).toHaveBeenCalledTimes(4)
    expect(listDirectoryMock.mock.calls.map(([path]) => path)).toEqual(['', 'book1', '', 'book1'])
  })

  it('emits a selected file from a loaded directory', async () => {
    const wrapper = mount(FileTree, { props: { selectedPath: markdown.path } })
    await flushPromises()
    await wrapper.get('.tree-row').trigger('click')
    await flushPromises()

    const rows = wrapper.findAll('.tree-row')
    await rows[1]?.trigger('click')

    expect(wrapper.emitted('open')).toEqual([[markdown]])
    expect(rows[1]?.classes()).toContain('selected')
  })
})
