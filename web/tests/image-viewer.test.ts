import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import type { FsItem } from '@/api/types'
import ImageViewer from '@/components/ImageViewer.vue'

vi.mock('@/api/client', () => ({
  AUTH_EXPIRED_EVENT: 'web-reader:auth-expired',
  getSession: vi.fn().mockResolvedValue({ authenticated: true }),
  rawFileUrl: (path: string, download = false, extraParams?: Record<string, string>) => {
    const params = new URLSearchParams({ path, ...(extraParams || {}) })
    if (download) params.set('download', '1')
    return `/api/fs/raw?${params.toString()}`
  },
}))

const mockItem: FsItem = {
  name: 'photo.png',
  path: 'gallery/photo.png',
  kind: 'file',
  previewKind: 'image',
  mime: 'image/png',
  size: 1024,
  modifiedAt: '2026-08-17T00:00:00Z',
}

describe('ImageViewer component', () => {
  it('renders image loading state initially and img element with correct source', () => {
    const wrapper = mount(ImageViewer, {
      props: { item: mockItem },
    })

    expect(wrapper.find('.image-loading').exists()).toBe(true)
    const img = wrapper.find('.image-viewer-img')
    expect(img.exists()).toBe(true)
    expect(img.attributes('src')).toBe('/api/fs/raw?path=gallery%2Fphoto.png')
  })

  it('updates image source when error occurs and retry button is clicked', async () => {
    const wrapper = mount(ImageViewer, {
      props: { item: mockItem },
    })

    const img = wrapper.find('.image-viewer-img')
    await img.trigger('error')

    expect(wrapper.find('.preview-state.error').exists()).toBe(true)
    expect(wrapper.find('h2').text()).toBe('图片加载失败')

    const retryBtn = wrapper.find('button.secondary-button')
    await retryBtn.trigger('click')

    expect(wrapper.find('.preview-state.error').exists()).toBe(false)
    expect(wrapper.find('.image-viewer-img').attributes('src')).toBe(
      '/api/fs/raw?path=gallery%2Fphoto.png&retry=1',
    )
  })
})
