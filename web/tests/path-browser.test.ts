import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import PathBrowser from '@/components/PathBrowser.vue'

const browseMock = vi.hoisted(() => vi.fn())

vi.mock('@/api/client', () => ({
  browseDirectories: browseMock,
}))

function mockResponse(path: string, dirs: { name: string; path: string }[]) {
  return { path, dirs }
}

describe('PathBrowser', () => {
  it('loads the initial path and renders breadcrumb + subdirectories', async () => {
    browseMock.mockResolvedValue(
      mockResponse('/home/user', [
        { name: 'docs', path: '/home/user/docs' },
        { name: 'workspace', path: '/home/user/workspace' },
      ]),
    )
    const wrapper = mount(PathBrowser, { props: { initialPath: '/home/user' } })
    await flushPromises()

    const crumbs = wrapper.findAll('.crumb-btn')
    expect(crumbs.length).toBe(3)
    expect(crumbs[0]!.text()).toBe('/')
    expect(crumbs[1]!.text()).toBe('home')
    expect(crumbs[2]!.text()).toBe('user')

    const items = wrapper.findAll('.dir-item')
    expect(items).toHaveLength(2)
    expect(items[0]!.text()).toContain('docs')
    expect(items[1]!.text()).toContain('workspace')
  })

  it('navigates into a subdirectory on click', async () => {
    browseMock.mockResolvedValueOnce(
      mockResponse('/home/user', [{ name: 'docs', path: '/home/user/docs' }]),
    )
    browseMock.mockResolvedValueOnce(
      mockResponse('/home/user/docs', [{ name: 'file', path: '/home/user/docs/file' }]),
    )

    const wrapper = mount(PathBrowser, { props: { initialPath: '/home/user' } })
    await flushPromises()

    await wrapper.findAll('.dir-item')[0]!.trigger('click')
    await flushPromises()

    expect(browseMock).toHaveBeenLastCalledWith('/home/user/docs')
    expect(wrapper.findAll('.crumb-btn').at(-1)!.text()).toBe('docs')
  })

  it('emits select with the current path', async () => {
    browseMock.mockResolvedValue(mockResponse('/srv/books', []))
    const wrapper = mount(PathBrowser, { props: { initialPath: '/srv/books' } })
    await flushPromises()

    await wrapper.find('.primary-button').trigger('click')
    expect(wrapper.emitted('select')).toEqual([['/srv/books']])
  })

  it('emits cancel when the cancel button is clicked', async () => {
    browseMock.mockResolvedValue(mockResponse('/srv/books', []))
    const wrapper = mount(PathBrowser, { props: { initialPath: '/srv/books' } })
    await flushPromises()

    await wrapper.find('.secondary-button').trigger('click')
    expect(wrapper.emitted('cancel')).toHaveLength(1)
  })

  it('shows an error message when the path is invalid', async () => {
    browseMock.mockRejectedValue(new Error('path not found'))
    const wrapper = mount(PathBrowser, { props: { initialPath: '/bad/path' } })
    await flushPromises()

    expect(wrapper.find('.browser-state.error').text()).toContain('path not found')
  })
})
