import { afterEach, describe, expect, it, vi } from 'vitest'
import { listDirectory } from '@/api/client'

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('API client cancellation', () => {
  it('preserves AbortError so callers can ignore stale requests', async () => {
    const controller = new AbortController()
    vi.stubGlobal(
      'fetch',
      vi.fn(
        (_input: RequestInfo | URL, init?: RequestInit) =>
          new Promise<Response>((_resolve, reject) => {
            init?.signal?.addEventListener('abort', () => {
              reject(new DOMException('The operation was aborted', 'AbortError'))
            })
          }),
      ),
    )

    const request = listDirectory('', controller.signal)
    controller.abort()

    await expect(request).rejects.toMatchObject({ name: 'AbortError' })
  })
})
