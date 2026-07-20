import type { FsItem, PreviewKind } from '@/api/types'

export function getPreviewMode(item: FsItem | null): PreviewKind | 'empty' {
  if (!item || item.kind !== 'file') return 'empty'
  return item.previewKind
}

export function isTextPreview(kind: PreviewKind): boolean {
  return kind === 'markdown' || kind === 'text'
}
