import { mount, flushPromises } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import type { TabItem } from '@/api/types'
import DocumentTabs from '@/components/DocumentTabs.vue'

describe('DocumentTabs', () => {
  const sampleTabs: TabItem[] = [
    { path: 'doc1.md', name: 'doc1.md', previewKind: 'markdown', pinned: true },
    { path: 'code.ts', name: 'code.ts', previewKind: 'text', pinned: false },
    { path: 'photo.png', name: 'photo.png', previewKind: 'image', pinned: false },
  ]

  it('renders tabs with correct active and pinned states', () => {
    const wrapper = mount(DocumentTabs, {
      props: {
        tabs: sampleTabs,
        activePath: 'code.ts',
        workspaceRoot: '/workspace',
      },
    })

    const tabElements = wrapper.findAll('.tab-item')
    expect(tabElements).toHaveLength(3)

    expect(tabElements[0]?.classes()).toContain('pinned')
    expect(tabElements[0]?.find('.tab-pin-icon').exists()).toBe(true)

    expect(tabElements[1]?.classes()).toContain('active')
    expect(tabElements[1]?.find('.tab-close-btn').exists()).toBe(true)

    expect(tabElements[2]?.classes()).not.toContain('active')
    expect(tabElements[2]?.classes()).not.toContain('pinned')
  })

  it('emits select when a tab is clicked', async () => {
    const wrapper = mount(DocumentTabs, {
      props: {
        tabs: sampleTabs,
        activePath: 'code.ts',
        workspaceRoot: '/workspace',
      },
    })

    const firstTab = wrapper.findAll('.tab-item')[0]
    await firstTab?.trigger('click')

    expect(wrapper.emitted('select')).toEqual([['doc1.md']])
  })

  it('emits close when close button is clicked', async () => {
    const wrapper = mount(DocumentTabs, {
      props: {
        tabs: sampleTabs,
        activePath: 'code.ts',
        workspaceRoot: '/workspace',
      },
    })

    const secondTabCloseBtn = wrapper.findAll('.tab-item')[1]?.find('.tab-close-btn')
    expect(secondTabCloseBtn?.exists()).toBe(true)
    await secondTabCloseBtn?.trigger('click')

    expect(wrapper.emitted('close')).toEqual([['code.ts']])
  })

  it('emits close on middle click (auxclick with button 1)', async () => {
    const wrapper = mount(DocumentTabs, {
      props: {
        tabs: sampleTabs,
        activePath: 'code.ts',
        workspaceRoot: '/workspace',
      },
    })

    const thirdTab = wrapper.findAll('.tab-item')[2]
    await thirdTab?.trigger('auxclick', { button: 1 })

    expect(wrapper.emitted('close')).toEqual([['photo.png']])
  })

  it('opens context menu on right click and handles actions', async () => {
    const wrapper = mount(DocumentTabs, {
      props: {
        tabs: sampleTabs,
        activePath: 'code.ts',
        workspaceRoot: '/workspace',
      },
      attachTo: document.body,
    })

    const secondTab = wrapper.findAll('.tab-item')[1]
    await secondTab?.trigger('contextmenu')
    await flushPromises()

    const contextMenu = wrapper.findComponent({ name: 'ContextMenu' })
    expect(contextMenu.exists()).toBe(true)

    const items = contextMenu.props('items') as Array<{ label: string; action: () => void }>
    const closeOthersItem = items.find((i) => i.label === '关闭其他标签页')
    expect(closeOthersItem).toBeDefined()
    closeOthersItem?.action()

    expect(wrapper.emitted('closeOthers')).toEqual([['code.ts']])

    const pinItem = items.find((i) => i.label === '固定标签页')
    expect(pinItem).toBeDefined()
    pinItem?.action()

    expect(wrapper.emitted('togglePin')).toEqual([['code.ts']])
    wrapper.unmount()
  })

  it('opens more menu dropdown and triggers closeAll', async () => {
    const wrapper = mount(DocumentTabs, {
      props: {
        tabs: sampleTabs,
        activePath: 'code.ts',
        workspaceRoot: '/workspace',
        recentFiles: ['doc1.md', 'code.ts'],
      },
    })

    const actionBtn = wrapper.find('.tab-action-btn')
    expect(actionBtn.exists()).toBe(true)
    await actionBtn.trigger('click')

    const dropdown = wrapper.find('.tabs-dropdown')
    expect(dropdown.exists()).toBe(true)

    const closeAllBtn = dropdown.findAll('.tabs-dropdown-item')[0]
    await closeAllBtn?.trigger('click')

    expect(wrapper.emitted('closeAll')).toHaveLength(1)
  })
})
