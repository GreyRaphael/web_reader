export interface ReaderTarget {
  path: string
  hash: string
}

interface ReferenceParts {
  pathname: string
  hash: string
}

function splitReference(reference: string): ReferenceParts {
  const hashIndex = reference.indexOf('#')
  const beforeHash = hashIndex >= 0 ? reference.slice(0, hashIndex) : reference
  const rawHash = hashIndex >= 0 ? reference.slice(hashIndex + 1) : ''
  const queryIndex = beforeHash.indexOf('?')
  return {
    pathname: queryIndex >= 0 ? beforeHash.slice(0, queryIndex) : beforeHash,
    hash: decodeFragment(rawHash),
  }
}

export function decodeFragment(value: string): string {
  try {
    return decodeURIComponent(value)
  } catch {
    return value
  }
}

export function isRelativeReference(reference: string): boolean {
  const value = reference.trim()
  return (
    value.length > 0 &&
    !value.startsWith('#') &&
    !value.startsWith('/') &&
    !value.startsWith('//') &&
    !/^[a-z][a-z\d+.-]*:/i.test(value)
  )
}

export function resolveWorkspacePath(currentFile: string, reference: string): string | null {
  if (!isRelativeReference(reference)) return null

  const { pathname } = splitReference(reference)
  const decoded = decodeFragment(pathname).replaceAll('\\', '/')
  if (!decoded || decoded.includes('\0')) return null

  const base = currentFile.replaceAll('\\', '/').split('/').slice(0, -1).filter(Boolean)
  for (const part of decoded.split('/')) {
    if (!part || part === '.') continue
    if (part === '..') {
      if (base.length === 0) return null
      base.pop()
    } else {
      base.push(part)
    }
  }
  return base.join('/')
}

export function resolveReaderTarget(currentFile: string, reference: string): ReaderTarget | null {
  const { pathname, hash } = splitReference(reference)
  const path = resolveWorkspacePath(currentFile, reference)
  if (!path || !pathname) return null
  return { path, hash }
}
