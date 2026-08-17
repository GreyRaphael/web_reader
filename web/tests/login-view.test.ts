import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as client from '@/api/client'
import LoginView from '@/views/LoginView.vue'

vi.mock('@/api/client', () => ({
  login: vi.fn(),
}))

describe('LoginView component', () => {
  beforeEach(() => {
    window.matchMedia = vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }))
  })

  it('renders login form and initial error if provided', () => {
    const wrapper = mount(LoginView, {
      props: { initialError: '会话已过期' },
    })

    expect(wrapper.find('h1').text()).toBe('Web Reader')
    expect(wrapper.find('.form-error').text()).toBe('会话已过期')
  })

  it('validates empty username or password', async () => {
    const wrapper = mount(LoginView)
    const usernameInput = wrapper.find('#username')
    const passwordInput = wrapper.find('#password')

    await usernameInput.setValue('')
    await passwordInput.setValue('')
    await wrapper.find('form').trigger('submit.prevent')

    expect(wrapper.find('.form-error').text()).toBe('请输入用户名和密码')
    expect(client.login).not.toHaveBeenCalled()
  })

  it('submits credentials and emits authenticated event on success', async () => {
    vi.mocked(client.login).mockResolvedValueOnce({
      authenticated: true,
      username: 'admin',
    })

    const wrapper = mount(LoginView)
    await wrapper.find('#username').setValue('admin')
    await wrapper.find('#password').setValue('secret123')
    await wrapper.find('form').trigger('submit.prevent')

    expect(client.login).toHaveBeenCalledWith('admin', 'secret123')
    expect(wrapper.emitted('authenticated')).toBeTruthy()
    expect(wrapper.emitted('authenticated')![0]).toEqual([
      { authenticated: true, username: 'admin' },
    ])
  })

  it('displays error message when login fails', async () => {
    vi.mocked(client.login).mockRejectedValueOnce(new Error('密码错误'))

    const wrapper = mount(LoginView)
    await wrapper.find('#username').setValue('admin')
    await wrapper.find('#password').setValue('wrong-password')
    await wrapper.find('form').trigger('submit.prevent')

    expect(wrapper.find('.form-error').text()).toBe('密码错误')
  })
})
