import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import OutlinePanel from '@/components/OutlinePanel.vue'

describe('OutlinePanel', () => {
  it('normalizes indentation and exposes the active heading', () => {
    const wrapper = mount(OutlinePanel, {
      props: {
        headings: [
          { id: 'overview', title: 'Overview', level: 3 },
          { id: 'details', title: 'Details', level: 4 },
        ],
        activeId: 'details',
      },
    })

    const rows = wrapper.findAll('.outline-link')
    expect(rows[0]?.attributes('style')).toContain('--outline-depth: 0')
    expect(rows[1]?.attributes('style')).toContain('--outline-depth: 1')

    const activeTitleBtn = rows[1]?.find('.outline-title-btn')
    expect(activeTitleBtn?.attributes('aria-current')).toBe('location')
  })

  it('toggles child collapse via an accessible button with aria-expanded', async () => {
    const wrapper = mount(OutlinePanel, {
      props: {
        headings: [
          { id: 'parent', title: 'Parent', level: 2 },
          { id: 'child', title: 'Child', level: 3 },
        ],
        activeId: 'parent',
      },
    })

    const toggle = wrapper.find('.outline-chevron')
    expect(toggle.exists()).toBe(true)
    expect(toggle.attributes('aria-expanded')).toBe('true')

    await toggle.trigger('click')
    expect(wrapper.find('.outline-chevron').attributes('aria-expanded')).toBe('false')
  })
})
