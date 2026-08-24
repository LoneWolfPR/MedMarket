import { describe, expect, it } from 'vitest'
import schema from './registerSchema'

// A complete, valid submission. Every test starts from this and breaks exactly
// one thing, so a failure names the rule that broke rather than "the form".
function validValues(overrides: Record<string, string> = {}) {
  return {
    firstName: 'Ada',
    lastName: 'Lovelace',
    street1: '1 Analytical Way',
    street2: '',
    city: 'London',
    state: 'NY',
    zip: '10001',
    email: 'ada@example.com',
    phone: '555-123-4567',
    password: 'Password1!',
    confirmPassword: 'Password1!',
    ...overrides,
  }
}

// The no-address baseline: valid everywhere else, address entirely blank.
function emptyAddress(overrides: Record<string, string> = {}) {
  return validValues({ street1: '', street2: '', city: '', state: '', zip: '', ...overrides })
}

// The messages a given field collected, so assertions can name the rule that
// fired instead of depending on the order issues happen to come back in.
function errorsFor(values: Record<string, string>, field: string): string[] {
  const result = schema.safeParse(values)
  if (result.success) return []
  return result.error.issues.filter((i) => i.path[0] === field).map((i) => i.message)
}

function parsed(values: Record<string, string>) {
  const result = schema.safeParse(values)
  if (!result.success) throw new Error(`expected valid, got: ${result.error.issues[0]?.message}`)
  return result.data
}

describe('register schema', () => {
  it('accepts a complete valid submission', () => {
    expect(schema.safeParse(validValues()).success).toBe(true)
  })

  describe('names', () => {
    it.each(['firstName', 'lastName'])('rejects an empty %s', (field) => {
      expect(errorsFor(validValues({ [field]: '' }), field)).toContain('Required')
    })

    // The backend trims before checking, so whitespace-only would pass a bare
    // .min(1) here and then 400 server-side. .trim() runs first, closing that gap.
    it.each(['firstName', 'lastName'])('rejects a whitespace-only %s', (field) => {
      expect(errorsFor(validValues({ [field]: '   ' }), field)).toContain('Required')
    })

    it('trims the value it hands back', () => {
      expect(parsed(validValues({ firstName: '  Ada  ' })).firstName).toBe('Ada')
    })
  })

  describe('password', () => {
    // Each case violates exactly one rule, which is what makes the per-rule
    // messages trustworthy: a wrong pattern surfaces as the wrong message rather
    // than as a generic failure.
    const cases: [string, string, string][] = [
      ['too short', 'Pass1!a', 'Password must be at least 8 characters'],
      ['too long', 'Password1!'.repeat(4), 'Password cannot be longer than 32 characters'],
      [
        'missing an uppercase letter',
        'password1!',
        'Password must contain an upper case character',
      ],
      ['missing a lowercase letter', 'PASSWORD1!', 'Password must contain a lower case character'],
      ['missing a digit', 'Passwords!', 'Password must contain a number'],
      ['missing a special character', 'Password12', 'Password must contain a special character'],
      ['holding a disallowed character', 'Password 1!', 'Password contains an invalid character'],
    ]

    it.each(cases)('rejects a password %s', (_name, password, message) => {
      expect(errorsFor(validValues({ password, confirmPassword: password }), 'password')).toContain(
        message,
      )
    })

    // Mirrors user.AllowedSpecialChars in the backend. A character allowed there
    // and rejected here would fail the form for a password the server accepts.
    it('accepts every character in the backend allowed set', () => {
      const specials = '`!@#$%^&*()_-+=[]{}|\\:;"\'<,>.?/~'
      for (const c of specials) {
        const password = `Passw12${c}`
        const values = validValues({ password, confirmPassword: password })
        expect(errorsFor(values, 'password'), `rejected ${c}`).toEqual([])
      }
    })

    // A password is used verbatim, so surrounding whitespace is rejected rather
    // than silently trimmed away — trimming would authenticate against something
    // the user never typed. The allowlist is what enforces it.
    it('rejects surrounding whitespace instead of trimming it', () => {
      const password = ' Password1! '
      expect(errorsFor(validValues({ password, confirmPassword: password }), 'password')).toContain(
        'Password contains an invalid character',
      )
    })

    it('reports a mismatch on confirmPassword, not password', () => {
      const values = validValues({ confirmPassword: 'Different1!' })
      expect(errorsFor(values, 'confirmPassword')).toContain('Passwords do not match')
      expect(errorsFor(values, 'password')).toEqual([])
    })
  })

  describe('phone', () => {
    it.each(['', '(555) 123-4567', '(555)123-4567', '555-123-4567', '555.555.4567', '5551234567'])(
      'accepts %j',
      (phone) => {
        expect(errorsFor(validValues({ phone }), 'phone')).toEqual([])
      },
    )

    // Space-separated is the shape people reach for first and the one shared.Phone
    // deliberately does not accept, so the form has to catch it here.
    it.each(['555 123 4567', '555-1234', '15551234567', '555-123-456', 'call me'])(
      'rejects %j',
      (phone) => {
        expect(errorsFor(validValues({ phone }), 'phone')).toContain(
          'Enter a phone number like 555-123-4567',
        )
      },
    )
  })

  describe('zip', () => {
    it('accepts five digits', () => {
      expect(errorsFor(validValues({ zip: '12345' }), 'zip')).toEqual([])
    })

    it('accepts an empty zip when the whole address is empty', () => {
      expect(errorsFor(emptyAddress(), 'zip')).toEqual([])
    })

    it.each(['1234', '123456', '1234a'])('rejects %j', (zip) => {
      expect(errorsFor(validValues({ zip }), 'zip')).toContain('Zip code must be 5 numbers')
    })
  })

  describe('address group', () => {
    it('accepts a submission with no address at all', () => {
      expect(schema.safeParse(emptyAddress()).success).toBe(true)
    })

    it('accepts a complete address without street2', () => {
      expect(schema.safeParse(validValues({ street2: '' })).success).toBe(true)
    })

    it('requires the rest once street1 is filled', () => {
      const values = emptyAddress({ street1: '1 Analytical Way' })
      expect(errorsFor(values, 'city')).toContain('City is missing')
      expect(errorsFor(values, 'state')).toContain('State is missing')
      expect(errorsFor(values, 'zip')).toContain('Zip is missing')
      expect(errorsFor(values, 'street1')).toEqual([])
    })

    // street2 alone still counts as starting an address — someone who typed only
    // an apartment number should be told what else is needed, not silently sent
    // a request carrying no address.
    it('requires the rest when only street2 is filled', () => {
      const values = emptyAddress({ street2: 'Apt 4' })
      expect(errorsFor(values, 'street1')).toContain('Street 1 is missing')
      expect(errorsFor(values, 'city')).toContain('City is missing')
      expect(errorsFor(values, 'state')).toContain('State is missing')
      expect(errorsFor(values, 'zip')).toContain('Zip is missing')
    })

    // Whitespace is trimmed before the group check, so spaces cannot stand in for
    // a real value and produce a half-filled address server-side.
    it('does not let whitespace satisfy a group member', () => {
      const values = emptyAddress({
        street1: '1 Analytical Way',
        city: '   ',
        state: 'NY',
        zip: '10001',
      })
      expect(errorsFor(values, 'city')).toContain('City is missing')
    })
  })
})
