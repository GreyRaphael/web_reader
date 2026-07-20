import { describe, expect, it } from 'vitest'
import { rawFileUrl } from '@/api/client'
import { resolveReaderTarget, resolveWorkspacePath } from '@/utils/path'

describe('workspace path rewriting', () => {
  it('resolves relative paths against the current document', () => {
    expect(resolveWorkspacePath('guide/intro/start.md', '../images/cover one.png')).toBe(
      'guide/images/cover one.png',
    )
    expect(resolveWorkspacePath('guide/start.md', './notes.txt')).toBe('guide/notes.txt')
  })

  it('does not resolve external, absolute, or escaping paths', () => {
    expect(resolveWorkspacePath('guide/start.md', 'https://example.com/a.png')).toBeNull()
    expect(resolveWorkspacePath('guide/start.md', '/root/a.png')).toBeNull()
    expect(resolveWorkspacePath('start.md', '../secret.txt')).toBeNull()
  })

  it('recognizes workspace file links and preserves fragments', () => {
    expect(resolveReaderTarget('guide/start.md', './chapter%202.md#API%20说明')).toEqual({
      path: 'guide/chapter 2.md',
      hash: 'API 说明',
    })
    expect(resolveReaderTarget('guide/start.md', './archive.zip')).toEqual({
      path: 'guide/archive.zip',
      hash: '',
    })
    expect(resolveReaderTarget('guide/start.md', './config.json')).toEqual({
      path: 'guide/config.json',
      hash: '',
    })
  })

  it('encodes authenticated raw and download URLs', () => {
    expect(rawFileUrl('guide/images/cover one.png')).toBe(
      '/api/fs/raw?path=guide%2Fimages%2Fcover+one.png',
    )
    expect(rawFileUrl('a.txt', true)).toBe('/api/fs/raw?path=a.txt&download=1')
  })
})
