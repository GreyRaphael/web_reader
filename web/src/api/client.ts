import type { FileListResponse, FileMetaResponse, SessionResponse, TextResponse } from './types'

export const AUTH_EXPIRED_EVENT = 'web-reader:auth-expired'

export class ApiError extends Error {
  readonly status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

function apiUrl(endpoint: string, params?: Record<string, string>): string {
  const query = new URLSearchParams(params)
  const suffix = query.size > 0 ? `?${query.toString()}` : ''
  return `/api${endpoint}${suffix}`
}

async function request<T>(endpoint: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  headers.set('Accept', 'application/json')
  if (init.body !== undefined) {
    headers.set('Content-Type', 'application/json')
  }

  let response: Response
  try {
    response = await fetch(endpoint, {
      ...init,
      headers,
      credentials: 'same-origin',
    })
  } catch (error) {
    if (
      typeof error === 'object' &&
      error !== null &&
      'name' in error &&
      error.name === 'AbortError'
    ) {
      throw error
    }
    throw new ApiError(error instanceof Error ? error.message : '网络连接失败', 0)
  }

  if (!response.ok) {
    let message = `请求失败（${response.status}）`
    try {
      const payload = (await response.json()) as { error?: string; message?: string }
      message = payload.message || payload.error || message
    } catch {
      const text = await response.text().catch(() => '')
      if (text.trim()) message = text.trim()
    }

    if (response.status === 401 && typeof window !== 'undefined') {
      window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT))
    }
    throw new ApiError(message, response.status)
  }

  if (response.status === 204) return undefined as T
  return (await response.json()) as T
}

export async function getSession(): Promise<SessionResponse> {
  return request<SessionResponse>(apiUrl('/auth/session'))
}

export async function login(username: string, password: string): Promise<SessionResponse> {
  await request<unknown>(apiUrl('/auth/login'), {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  })
  return getSession()
}

export async function logout(): Promise<void> {
  await request<void>(apiUrl('/auth/logout'), {
    method: 'POST',
    body: JSON.stringify({}),
  })
}

export async function listDirectory(path: string, signal?: AbortSignal): Promise<FileListResponse> {
  return request<FileListResponse>(apiUrl('/fs/list', { path }), { signal })
}

export async function getFileMeta(path: string, signal?: AbortSignal): Promise<FileMetaResponse> {
  return request<FileMetaResponse>(apiUrl('/fs/meta', { path }), { signal })
}

export async function getTextFile(path: string, signal?: AbortSignal): Promise<TextResponse> {
  return request<TextResponse>(apiUrl('/fs/text', { path }), { signal })
}

export function rawFileUrl(path: string, download = false): string {
  return apiUrl('/fs/raw', download ? { path, download: '1' } : { path })
}

export async function createFile(path: string): Promise<FileMetaResponse> {
  return request<FileMetaResponse>(apiUrl('/fs/file'), {
    method: 'POST',
    body: JSON.stringify({ path }),
  })
}

export async function createDir(path: string): Promise<FileMetaResponse> {
  return request<FileMetaResponse>(apiUrl('/fs/dir'), {
    method: 'POST',
    body: JSON.stringify({ path }),
  })
}

export async function uploadFile(path: string, body: ArrayBuffer): Promise<FileMetaResponse> {
  const headers = new Headers()
  headers.set('Accept', 'application/json')
  headers.set('Content-Type', 'application/octet-stream')
  return request<FileMetaResponse>(apiUrl('/fs/upload', { path }), {
    method: 'POST',
    headers,
    body,
    credentials: 'same-origin',
  })
}

export async function renameFile(path: string, newName: string): Promise<FileMetaResponse> {
  return request<FileMetaResponse>(apiUrl('/fs/rename'), {
    method: 'POST',
    body: JSON.stringify({ path, newName }),
  })
}

export async function deleteFile(path: string): Promise<{ deleted: string }> {
  return request<{ deleted: string }>(apiUrl('/fs/delete', { path }), {
    method: 'DELETE',
  })
}
