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

describe('Update API client', () => {
  it('calls checkUpdate and parses response correctly', async () => {
    const mockRes = {
      currentVersion: 'v1.7.1',
      latestVersion: 'v1.7.2',
      hasUpdate: true,
      canAutoUpdate: true,
    }
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: async () => mockRes,
      }),
    )

    const { checkUpdate } = await import('@/api/client')
    const res = await checkUpdate()
    expect(res).toEqual(mockRes)
  })

  it('calls applyUpdate with POST method', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: 'success', message: 'updated' }),
    })
    vi.stubGlobal('fetch', fetchMock)

    const { applyUpdate } = await import('@/api/client')
    const res = await applyUpdate()
    expect(res.status).toBe('success')
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/update/apply',
      expect.objectContaining({ method: 'POST' }),
    )
  })

  it('checkHealth returns true on 200 ok and false on network failure', async () => {
    const { checkHealth } = await import('@/api/client')

    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true }))
    expect(await checkHealth()).toBe(true)

    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')))
    expect(await checkHealth()).toBe(false)
  })
})
