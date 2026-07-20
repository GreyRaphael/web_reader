import type { FsItem } from '@/api/types'

export function sortFileItems(items: FsItem[]): FsItem[] {
  return [...items].sort((left, right) => {
    if (left.kind !== right.kind) return left.kind === 'directory' ? -1 : 1
    return left.name.localeCompare(right.name, undefined, {
      numeric: true,
      sensitivity: 'base',
    })
  })
}
