const usd = new Intl.NumberFormat('en-US', {
  style: 'currency',
  currency: 'USD',
})

/**
 * Formats an integer cent amount as US currency: 1299 -> "$12.99".
 *
 * The API carries money as integer cents precisely so no float ever holds a
 * price, so the division happens here, at the one place a price becomes display
 * text, and nowhere else.
 */
export function formatCents(cents: number): string {
  return usd.format(cents / 100)
}
