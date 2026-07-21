import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import FileTree from '@/components/FileTree.vue'
import type { FsItem } from '@/api/types'

const listDirectoryMock = vi.hoisted(() => vi.fn())

vi.mock('@/api/client', () => ({
  listDirectory: listDirectoryMock,
  createFile: vi.fn(),
  createDir: vi.fn(),
  deleteFile: vi.fn(),
  moveFile: vi.fn(),
  uploadFile: vi.fn(),
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
    const win = window as unknown as { matchMedia: () => { matches: boolean } }
    win.matchMedia = () => ({ matches: false })
  })

  it('loads root directory on mount and navigates on directory click', async () => {
    const wrapper = mount(FileTree, { props: { selectedPath: '' } })
    await flushPromises()

    expect(listDirectoryMock).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('book1')
    expect(wrapper.text()).toContain('~')

    await wrapper.find('.tree-label').trigger('click')
    await flushPromises()

    expect(listDirectoryMock).toHaveBeenCalledTimes(2)
    expect(listDirectoryMock).toHaveBeenLastCalledWith('book1', expect.any(AbortSignal))
    expect(wrapper.text()).toContain('chapter1.md')
  })

  it('emits open when a file is clicked', async () => {
    listDirectoryMock.mockImplementation(async () => ({ items: [markdown] }))
    const wrapper = mount(FileTree, { props: { selectedPath: '' } })
    await flushPromises()

    await wrapper.find('.tree-label').trigger('click')
    expect(wrapper.emitted('open')).toEqual([[markdown]])
  })

  it('shows breadcrumb and navigates via breadcrumb', async () => {
    const wrapper = mount(FileTree, { props: { selectedPath: '' } })
    await flushPromises()

    expect(wrapper.findAll('.bc-crumb').length).toBe(1)
    expect(wrapper.find('.bc-crumb').text()).toBe('~')

    await wrapper.find('.tree-label').trigger('click')
    await flushPromises()

    const crumbs = wrapper.findAll('.bc-crumb')
    expect(crumbs.length).toBe(2)
    expect(crumbs[1]?.text()).toBe('book1')

    await crumbs[0]!.trigger('click')
    await flushPromises()

    expect(wrapper.findAll('.bc-crumb').length).toBe(1)
  })

  it('expands directory inline when chevron clicked', async () => {
    const wrapper = mount(FileTree, { props: { selectedPath: '' } })
    await flushPromises()

    const chevron = wrapper.find('.tree-chevron')
    await chevron.trigger('click')
    await flushPromises()

    expect(listDirectoryMock).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('chapter1.md')
  })

  it('clicking file from expanded subtree opens it without navigating', async () => {
    const wrapper = mount(FileTree, { props: { selectedPath: '' } })
    await flushPromises()

    await wrapper.find('.tree-chevron').trigger('click')
    await flushPromises()

    const childRow = wrapper.find('.tree-child-row')
    await childRow.trigger('click')

    expect(wrapper.emitted('open')).toEqual([[markdown]])
    expect(wrapper.findAll('.bc-crumb').length).toBe(1)
  })
})