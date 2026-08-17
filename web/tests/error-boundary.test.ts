import { mount } from '@vue/test-utils'
import { defineComponent, h, nextTick } from 'vue'
import { describe, expect, it } from 'vitest'
import ErrorBoundary from '@/components/ErrorBoundary.vue'

const GoodComponent = defineComponent({
  name: 'GoodComponent',
  render() {
    return h('div', { class: 'good-child' }, 'Normal Content')
  },
})

const BrokenComponent = defineComponent({
  name: 'BrokenComponent',
  mounted() {
    throw new Error('Test Crash Error')
  },
  render() {
    return h('div', 'Broken')
  },
})

const ParentComponent = defineComponent({
  name: 'ParentComponent',
  components: { ErrorBoundary, BrokenComponent, GoodComponent },
  props: {
    hasError: { type: Boolean, default: false },
    resetKey: { type: String, default: '' },
  },
  template: `
    <ErrorBoundary :reset-key="resetKey">
      <BrokenComponent v-if="hasError" />
      <GoodComponent v-else />
    </ErrorBoundary>
  `,
})

describe('ErrorBoundary component', () => {
  it('renders default slot content when no error occurs', () => {
    const wrapper = mount(ParentComponent, {
      props: { hasError: false },
    })

    expect(wrapper.find('.good-child').exists()).toBe(true)
    expect(wrapper.find('.error-boundary').exists()).toBe(false)
  })

  it('catches child error and displays fallback UI', async () => {
    const wrapper = mount(ParentComponent, {
      props: { hasError: false },
    })

    await wrapper.setProps({ hasError: true })
    await nextTick()

    expect(wrapper.find('.error-boundary').exists()).toBe(true)
    expect(wrapper.text()).toContain('Test Crash Error')
    expect(wrapper.find('button').text()).toBe('重试')
  })

  it('resets error state when resetKey prop changes', async () => {
    const wrapper = mount(ParentComponent, {
      props: { hasError: false, resetKey: 'user-1' },
    })

    await wrapper.setProps({ hasError: true })
    await nextTick()
    expect(wrapper.find('.error-boundary').exists()).toBe(true)

    await wrapper.setProps({ hasError: false, resetKey: 'user-2' })
    await nextTick()

    expect(wrapper.find('.good-child').exists()).toBe(true)
    expect(wrapper.find('.error-boundary').exists()).toBe(false)
  })
})
