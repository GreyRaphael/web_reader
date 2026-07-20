import { describe, expect, it } from 'vitest'
import type { FsItem } from '@/api/types'
import { getPreviewMode } from '@/utils/preview'
import { createUniqueSlugger } from '@/utils/slug'

function item(previewKind: FsItem['previewKind'], kind: FsItem['kind'] = 'file'): FsItem {
  return {
    path: 'sample',
    name: 'sample',
    kind,
    previewKind,
    size: 0,
    modifiedAt: '',
    mime: '',
  }
}

describe('preview dispatch', () => {
  it.each(['markdown', 'text', 'image', 'unsupported'] as const)('dispatches %s files', (kind) =>
    expect(getPreviewMode(item(kind))).toBe(kind),
  )

  it('does not preview directories', () => {
    expect(getPreviewMode(item('text', 'directory'))).toBe('empty')
    expect(getPreviewMode(null)).toBe('empty')
  })
})

describe('heading slugger', () => {
  it('increments duplicates independently', () => {
    const slug = createUniqueSlugger()
    expect([slug('API'), slug('API'), slug('API!'), slug('Other')]).toEqual([
      'api',
      'api-1',
      'api-2',
      'other',
    ])
  })
})
