import { describe, it, expect } from 'vitest'

import { formatCents } from './money'

describe('formatCents', () => {
  it.each([
    [0, '$0.00'],
    [5, '$0.05'],
    [99, '$0.99'],
    [1299, '$12.99'],
    // A whole-dollar amount still shows both decimal places.
    [1200, '$12.00'],
    // Thousands separator above $999.
    [123456, '$1,234.56'],
    [100000000, '$1,000,000.00'],
  ])('formats %i cents as %s', (cents, want) => {
    expect(formatCents(cents)).toBe(want)
  })
})
