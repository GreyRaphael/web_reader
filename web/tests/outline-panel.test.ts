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

    const links = wrapper.findAll('.outline-link')
    expect(links[0]?.attributes('style')).toContain('--outline-depth: 0')
    expect(links[1]?.attributes('style')).toContain('--outline-depth: 1')
    expect(links[1]?.attributes('aria-current')).toBe('location')
  })
})
