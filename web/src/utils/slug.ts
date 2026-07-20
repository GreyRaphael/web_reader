export function slugifyHeading(value: string): string {
  const slug = value
    .normalize('NFKC')
    .trim()
    .toLocaleLowerCase()
    .replace(/[\s_]+/gu, '-')
    .replace(/[^\p{L}\p{N}-]/gu, '')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
  return slug || 'section'
}

export function createUniqueSlugger(): (value: string) => string {
  const counts = new Map<string, number>()
  return (value: string) => {
    const base = slugifyHeading(value)
    const count = counts.get(base) ?? 0
    counts.set(base, count + 1)
    return count === 0 ? base : `${base}-${count}`
  }
}
