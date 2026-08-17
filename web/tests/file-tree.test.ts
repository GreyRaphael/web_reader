import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import FileTree from '@/components/FileTree.vue'
import type { FsItem } from '@/api/types'

const listDirectoryMock = vi.hoisted(() => vi.fn())
const getWorkspaceMock = vi.hoisted(() => vi.fn())
const uploadFileMock = vi.hoisted(() => vi.fn())

vi.mock('@/api/client', () => ({
  listDirectory: listDirectoryMock,
  createFile: vi.fn(),
  createDir: vi.fn(),
  deleteFile: vi.fn(),
  moveFile: vi.fn(),
  uploadFile: uploadFileMock,
  renameFile: vi.fn(),
  getWorkspace: getWorkspaceMock,
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
    getWorkspaceMock.mockReset()
    getWorkspaceMock.mockResolvedValue({ workspace: '/home/user/workspace' })
    uploadFileMock.mockReset()
    uploadFileMock.mockResolvedValue({ item: markdown })
    File.prototype.arrayBuffer = vi.fn().mockResolvedValue(new ArrayBuffer(8))
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

  it('clicking directory row from expanded subtree navigates into it as root', async () => {
    const subDir: FsItem = {
      path: 'book1/subfolder',
      name: 'subfolder',
      kind: 'directory',
      previewKind: 'unsupported',
      size: 0,
      modifiedAt: '2026-07-20T00:00:00Z',
      mime: '',
    }
    listDirectoryMock.mockImplementation(async (path: string) => ({
      items: path === '' ? [directory] : path === 'book1' ? [subDir] : [markdown],
    }))

    const wrapper = mount(FileTree, { props: { selectedPath: '' } })
    await flushPromises()

    // Expand book1 inline
    await wrapper.find('.tree-chevron').trigger('click')
    await flushPromises()

    // Click on the child directory row
    const childRow = wrapper.find('.tree-child-row')
    expect(childRow.text()).toContain('subfolder')
    await childRow.trigger('click')
    await flushPromises()

    // Should have navigated to book1/subfolder as root
    expect(listDirectoryMock).toHaveBeenLastCalledWith('book1/subfolder', expect.any(AbortSignal))
    const crumbs = wrapper.findAll('.bc-crumb')
    expect(crumbs.length).toBe(3)
    expect(crumbs[2]?.text()).toBe('subfolder')
  })

  it('opens context menu and copies absolute path', async () => {
    const writeTextMock = vi.fn()
    Object.assign(navigator, {
      clipboard: { writeText: writeTextMock },
    })

    const wrapper = mount(FileTree, { props: { selectedPath: '' } })
    await flushPromises()

    const labelBtn = wrapper.find('.tree-label')
    await labelBtn.trigger('contextmenu')
    await flushPromises()

    const contextMenu = wrapper.findComponent({ name: 'ContextMenu' })
    expect(contextMenu.exists()).toBe(true)

    const absCopyItem = contextMenu
      .props('items')
      .find((i: { label: string }) => i.label === '复制绝对路径')
    expect(absCopyItem).toBeDefined()
    await absCopyItem.action()

    expect(writeTextMock).toHaveBeenCalledWith('/home/user/workspace/book1')
  })

  it('reports full success when all batch files upload successfully', async () => {
    const wrapper = mount(FileTree, { props: { selectedPath: '' } })
    await flushPromises()

    const input = wrapper.find<HTMLInputElement>('.tree-file-input')
    const file1 = new File(['content1'], 'file1.txt', { type: 'text/plain' })
    const file2 = new File(['content2'], 'file2.txt', { type: 'text/plain' })

    Object.defineProperty(input.element, 'files', {
      value: [file1, file2],
      writable: true,
    })

    await input.trigger('change')
    await flushPromises()

    expect(uploadFileMock).toHaveBeenCalledTimes(2)
    expect(wrapper.find('.tree-tool-message').text()).toBe('成功上传 2 个文件')
  })

  it('reports partial failure when some files fail to upload', async () => {
    uploadFileMock
      .mockResolvedValueOnce({ item: markdown })
      .mockRejectedValueOnce(new Error('Network error'))

    const wrapper = mount(FileTree, { props: { selectedPath: '' } })
    await flushPromises()

    const input = wrapper.find<HTMLInputElement>('.tree-file-input')
    const file1 = new File(['content1'], 'file1.txt', { type: 'text/plain' })
    const file2 = new File(['content2'], 'file2.txt', { type: 'text/plain' })

    Object.defineProperty(input.element, 'files', {
      value: [file1, file2],
      writable: true,
    })

    await input.trigger('change')
    await flushPromises()

    expect(wrapper.find('.tree-tool-message').text()).toContain(
      '部分上传成功（1 成功，1 失败：file2.txt）',
    )
  })

  it('reports full failure when all files fail to upload', async () => {
    uploadFileMock.mockRejectedValue(new Error('Permission denied'))

    const wrapper = mount(FileTree, { props: { selectedPath: '' } })
    await flushPromises()

    const input = wrapper.find<HTMLInputElement>('.tree-file-input')
    const file1 = new File(['content1'], 'file1.txt', { type: 'text/plain' })

    Object.defineProperty(input.element, 'files', {
      value: [file1],
      writable: true,
    })

    await input.trigger('change')
    await flushPromises()

    expect(wrapper.find('.tree-tool-message').text()).toBe('上传失败 (1 个文件)：file1.txt')
  })

  it('cleans up SSE reconnect timer and eventSource on unmount', async () => {
    vi.useFakeTimers()
    const closeMock = vi.fn()
    class MockEventSource {
      close = closeMock
      onmessage: (() => void) | null = null
      onerror: (() => void) | null = null
      constructor() {
        setTimeout(() => {
          if (this.onerror) this.onerror()
        }, 10)
      }
    }
    const originalEventSource = globalThis.EventSource
    globalThis.EventSource = MockEventSource as unknown as typeof EventSource

    const wrapper = mount(FileTree, { props: { selectedPath: '' } })
    await flushPromises()

    vi.advanceTimersByTime(20)
    // Error triggered, reconnect timer set to 5000ms

    wrapper.unmount()
    // unmount should clear reconnect timer and event source
    expect(closeMock).toHaveBeenCalled()

    // advance beyond 5000ms, setupSSE should NOT run again
    const callCountBefore = closeMock.mock.calls.length
    vi.advanceTimersByTime(6000)
    expect(closeMock.mock.calls.length).toBe(callCountBefore)

    globalThis.EventSource = originalEventSource
    vi.useRealTimers()
  })

  it('uploads external files when dropped onto root scroll container', async () => {
    const wrapper = mount(FileTree, { props: { selectedPath: '' } })
    await flushPromises()

    const droppedFile = new File(['hello drop'], 'dropped.txt', { type: 'text/plain' })
    const scrollEl = wrapper.find('.tree-scroll')

    await scrollEl.trigger('dragover', {
      dataTransfer: {
        types: ['Files'],
        dropEffect: 'none',
      },
    })
    expect(wrapper.find('.tree-scroll').classes()).toContain('drag-over')

    await scrollEl.trigger('drop', {
      dataTransfer: {
        files: [droppedFile],
        getData: vi.fn(),
      },
    })
    await flushPromises()

    expect(uploadFileMock).toHaveBeenCalledWith('dropped.txt', expect.any(ArrayBuffer))
    expect(wrapper.find('.tree-tool-message').text()).toBe('成功上传 1 个文件')
  })

  it('uploads external files when dropped onto a specific directory node', async () => {
    const wrapper = mount(FileTree, { props: { selectedPath: '' } })
    await flushPromises()

    const droppedFile = new File(['folder drop'], 'doc.md', { type: 'text/markdown' })
    const dirNode = wrapper.find('.tree-node')

    await dirNode.trigger('dragover', {
      dataTransfer: {
        types: ['Files'],
        dropEffect: 'none',
      },
    })

    await dirNode.trigger('drop', {
      dataTransfer: {
        files: [droppedFile],
        getData: vi.fn(),
      },
    })
    await flushPromises()

    expect(uploadFileMock).toHaveBeenCalledWith('book1/doc.md', expect.any(ArrayBuffer))
    expect(wrapper.find('.tree-tool-message').text()).toBe('成功上传 1 个文件')
  })

  it('uploads external files when dropped onto breadcrumb', async () => {
    const wrapper = mount(FileTree, { props: { selectedPath: '' } })
    await flushPromises()

    // Navigate into book1
    await wrapper.find('.tree-label').trigger('click')
    await flushPromises()

    const droppedFile = new File(['crumb drop'], 'rootfile.txt', { type: 'text/plain' })
    const rootCrumb = wrapper.findAll('.bc-crumb')[0]
    expect(rootCrumb).toBeDefined()

    await rootCrumb!.trigger('dragover', {
      dataTransfer: {
        types: ['Files'],
        dropEffect: 'none',
      },
    })

    await rootCrumb!.trigger('drop', {
      dataTransfer: {
        files: [droppedFile],
        getData: vi.fn(),
      },
    })
    await flushPromises()

    expect(uploadFileMock).toHaveBeenCalledWith('rootfile.txt', expect.any(ArrayBuffer))
  })
})
