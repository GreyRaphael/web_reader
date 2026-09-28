import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import SettingsModal from '@/components/SettingsModal.vue'
import * as client from '@/api/client'

vi.mock('@/api/client', () => ({
  getSettings: vi.fn(),
  getWorkspace: vi.fn(),
  setWorkspace: vi.fn(),
  setSettings: vi.fn(),
  checkUpdate: vi.fn(),
  applyUpdate: vi.fn(),
  checkHealth: vi.fn(),
}))

describe('SettingsModal - Version and Update', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    HTMLDialogElement.prototype.showModal = vi.fn(function (this: HTMLDialogElement) {
      this.open = true
    })
    HTMLDialogElement.prototype.close = vi.fn(function (this: HTMLDialogElement) {
      this.open = false
    })

    vi.mocked(client.getSettings).mockResolvedValue({ enableTerminal: true })
    vi.mocked(client.getWorkspace).mockResolvedValue({ workspace: '~/workspace' })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders version section and check update button', async () => {
    const wrapper = mount(SettingsModal, {
      props: { username: 'testuser' },
    })
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('版本与更新 (Version & Update)')
    expect(wrapper.find('.check-update-btn').text()).toBe('检查更新')
  })

  it('displays up-to-date message when no newer version is found', async () => {
    vi.mocked(client.checkUpdate).mockResolvedValue({
      currentVersion: 'v1.7.1',
      latestVersion: 'v1.7.1',
      hasUpdate: false,
      releaseNotes: '',
      publishedAt: '2026-09-28T00:00:00Z',
      releaseUrl: 'https://github.com/GreyRaphael/web_reader/releases/tag/v1.7.1',
      assetName: 'web-reader-v1.7.1-linux-amd64.tar.gz',
      assetSize: 10485760,
      canAutoUpdate: true,
    })

    const wrapper = mount(SettingsModal, {
      props: { username: 'testuser' },
    })
    await wrapper.vm.$nextTick()

    const checkBtn = wrapper.find('.check-update-btn')
    await checkBtn.trigger('click')
    await wrapper.vm.$nextTick()

    expect(client.checkUpdate).toHaveBeenCalledTimes(1)
    expect(wrapper.find('.update-status-row.latest').text()).toContain('当前已是最新版本 (v1.7.1)')
  })

  it('displays new version info and toggle release notes when update is available', async () => {
    vi.mocked(client.checkUpdate).mockResolvedValue({
      currentVersion: 'v1.7.1',
      latestVersion: 'v1.7.2',
      hasUpdate: true,
      releaseNotes: '### 修复内容\n- 修复公式横向滚动条',
      publishedAt: '2026-09-28T12:00:00Z',
      releaseUrl: 'https://github.com/GreyRaphael/web_reader/releases/tag/v1.7.2',
      assetName: 'web-reader-v1.7.2-linux-amd64.tar.gz',
      assetSize: 15728640,
      canAutoUpdate: true,
    })

    const wrapper = mount(SettingsModal, {
      props: { username: 'testuser' },
    })
    await wrapper.vm.$nextTick()

    await wrapper.find('.check-update-btn').trigger('click')
    await wrapper.vm.$nextTick()

    expect(wrapper.find('.new-tag').text()).toBe('发现新版本')
    expect(wrapper.find('.latest-version-name').text()).toBe('v1.7.2')
    expect(wrapper.find('.update-meta').text()).toContain('15.0 MB')

    // Release notes toggle
    expect(wrapper.find('.release-notes-content').exists()).toBe(false)
    const toggleNotesBtn = wrapper.find('.notes-toggle-btn')
    await toggleNotesBtn.trigger('click')
    expect(wrapper.find('.release-notes-content').text()).toContain('修复公式横向滚动条')
    await toggleNotesBtn.trigger('click')
    expect(wrapper.find('.release-notes-content').exists()).toBe(false)
  })

  it('handles auto-update confirmation and applying update', async () => {
    vi.mocked(client.checkUpdate).mockResolvedValue({
      currentVersion: 'v1.7.1',
      latestVersion: 'v1.7.2',
      hasUpdate: true,
      releaseNotes: '新版本说明',
      publishedAt: '2026-09-28T12:00:00Z',
      releaseUrl: 'https://github.com/GreyRaphael/web_reader/releases/tag/v1.7.2',
      assetName: 'web-reader-v1.7.2-linux-amd64.tar.gz',
      assetSize: 15728640,
      canAutoUpdate: true,
    })
    vi.mocked(client.applyUpdate).mockResolvedValue({
      status: 'success',
      message: '更新成功，服务正在重启...',
      targetVersion: 'v1.7.2',
    })

    const wrapper = mount(SettingsModal, {
      props: { username: 'testuser' },
    })
    await wrapper.vm.$nextTick()

    await wrapper.find('.check-update-btn').trigger('click')
    await wrapper.vm.$nextTick()

    // Click apply update button -> shows confirmation banner
    const applyBtn = wrapper.find('.apply-update-btn')
    await applyBtn.trigger('click')
    expect(wrapper.find('.confirm-update-banner').exists()).toBe(true)

    // Cancel confirmation
    const cancelConfirmBtn = wrapper.find('.confirm-update-banner .secondary-button')
    await cancelConfirmBtn.trigger('click')
    expect(wrapper.find('.confirm-update-banner').exists()).toBe(false)

    // Re-trigger and confirm
    await wrapper.find('.apply-update-btn').trigger('click')
    const confirmBtn = wrapper.find('.confirm-update-banner .primary-button')
    await confirmBtn.trigger('click')
    await wrapper.vm.$nextTick()

    expect(client.applyUpdate).toHaveBeenCalledTimes(1)
    expect(wrapper.find('.restart-progress-card').text()).toContain('服务正在重启')
    expect(wrapper.find('.modal-footer .secondary-button').attributes('disabled')).toBeDefined()
  })

  it('displays warning when canAutoUpdate is false', async () => {
    vi.mocked(client.checkUpdate).mockResolvedValue({
      currentVersion: 'dev',
      latestVersion: 'v1.7.2',
      hasUpdate: true,
      releaseNotes: '',
      publishedAt: '2026-09-28T12:00:00Z',
      releaseUrl: 'https://github.com/GreyRaphael/web_reader/releases/tag/v1.7.2',
      assetName: '',
      assetSize: 0,
      canAutoUpdate: false,
      reason: '当前处于开发模式，请使用源码编译或手动安装',
    })

    const wrapper = mount(SettingsModal, {
      props: { username: 'testuser' },
    })
    await wrapper.vm.$nextTick()

    await wrapper.find('.check-update-btn').trigger('click')
    await wrapper.vm.$nextTick()

    expect(wrapper.find('.auto-update-warning').text()).toContain(
      '当前处于开发模式，请使用源码编译或手动安装',
    )
    expect(wrapper.find('.apply-update-btn').exists()).toBe(false)
  })

  it('displays error message when checkUpdate fails', async () => {
    vi.mocked(client.checkUpdate).mockRejectedValue(new Error('网络请求超时'))

    const wrapper = mount(SettingsModal, {
      props: { username: 'testuser' },
    })
    await wrapper.vm.$nextTick()

    await wrapper.find('.check-update-btn').trigger('click')
    await wrapper.vm.$nextTick()

    expect(wrapper.find('.version-update-section .alert-box.error').text()).toBe('网络请求超时')
  })
})
