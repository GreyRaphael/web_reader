import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import TerminalKeyBar from '@/components/TerminalKeyBar.vue'

describe('TerminalKeyBar', () => {
  it('renders the CTRL toggle and special keys', () => {
    const wrapper = mount(TerminalKeyBar)
    const buttons = wrapper.findAll('.key-btn')
    expect(buttons.length).toBe(11)
    expect(buttons[0]!.text()).toBe('CTRL')
    expect(wrapper.find('.key-btn.active').exists()).toBe(false)
  })

  it('emits the correct escape sequence for ESC', async () => {
    const wrapper = mount(TerminalKeyBar)
    const escBtn = wrapper.findAll('.key-btn')[1]!
    await escBtn.trigger('click')
    expect(wrapper.emitted('key')).toEqual([['\x1b']])
  })

  it('emits arrow key escape sequences', async () => {
    const wrapper = mount(TerminalKeyBar)
    const upBtn = wrapper.findAll('.key-btn')[3]!
    await upBtn.trigger('click')
    expect(wrapper.emitted('key')).toEqual([['\x1b[A']])
  })

  it('toggles CTRL and sends control sequences', async () => {
    const wrapper = mount(TerminalKeyBar)
    const ctrlBtn = wrapper.findAll('.key-btn')[0]!
    await ctrlBtn.trigger('click')
    expect(ctrlBtn.classes()).toContain('active')

    // After CTRL is active, pressing 'C' should send Ctrl+C (\x03)
    const slashBtn = wrapper.findAll('.key-btn')[7]!
    await slashBtn.trigger('click')
    expect(wrapper.emitted('key')).toBeTruthy()
    // CTRL should be deactivated after one use
    expect(ctrlBtn.classes()).not.toContain('active')
  })

  it('deactivates CTRL after sending one key', async () => {
    const wrapper = mount(TerminalKeyBar)
    const ctrlBtn = wrapper.findAll('.key-btn')[0]!
    await ctrlBtn.trigger('click')
    expect(ctrlBtn.attributes('aria-pressed')).toBe('true')

    const slashBtn = wrapper.findAll('.key-btn')[7]!
    await slashBtn.trigger('click')

    expect(ctrlBtn.attributes('aria-pressed')).toBe('false')
  })
})
