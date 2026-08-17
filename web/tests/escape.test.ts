import { describe, expect, it } from 'vitest'
import { escapeHtml } from '@/utils/escape'

describe('escapeHtml utility', () => {
  it('escapes special HTML characters', () => {
    expect(escapeHtml('<script>alert("XSS & fun")</script>')).toBe(
      '&lt;script&gt;alert(&quot;XSS &amp; fun&quot;)&lt;/script&gt;',
    )
    expect(escapeHtml("it's 'quoted'")).toBe('it&#039;s &#039;quoted&#039;')
  })

  it('handles plain string without special chars', () => {
    expect(escapeHtml('Hello world 123')).toBe('Hello world 123')
    expect(escapeHtml('')).toBe('')
  })
})
