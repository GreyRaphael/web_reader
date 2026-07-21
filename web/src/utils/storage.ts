export function storedBoolean(key: string, fallback: boolean): boolean {
  try {
    const value = window.localStorage.getItem(key)
    return value === null ? fallback : value === 'true'
  } catch {
    return fallback
  }
}

export function storedNumber(
  key: string,
  fallback: number,
  min: number,
  max: number,
): number {
  try {
    const value = Number(window.localStorage.getItem(key))
    return Number.isFinite(value) && value >= min && value <= max ? value : fallback
  } catch {
    return fallback
  }
}