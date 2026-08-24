import { describe, expect, it } from 'vitest'
import { readRedirect } from './locationState'

// readRedirect is the only thing standing between router state and a navigate
// call. Router state is typed `any` and survives in the browser's history entry,
// so it is an untrusted input in the same sense a response body is — these cases
// exist to keep the guard from quietly eroding into a property read.
describe('readRedirect', () => {
  it('returns an internal path', () => {
    expect(readRedirect({ from: '/prescriptions' })).toBe('/prescriptions')
  })

  it('returns the path when other keys ride along', () => {
    expect(readRedirect({ from: '/prescriptions', registered: true })).toBe('/prescriptions')
  })

  // The common case: navigating without state at all. `null` is what react-router
  // hands back, and `typeof null === 'object'`, so the null check is load-bearing
  // rather than defensive decoration.
  it.each([
    ['null', null],
    ['undefined', undefined],
    ['a string', '/prescriptions'],
    ['a number', 42],
    ['an array', ['/prescriptions']],
  ])('returns undefined for %s', (_name, state) => {
    expect(readRedirect(state)).toBeUndefined()
  })

  it.each([
    ['no from key', { registered: true }],
    ['a present-but-undefined from', { from: undefined }],
    ['a non-string from', { from: 42 }],
    ['a null from', { from: null }],
  ])('returns undefined for an object with %s', (_name, state) => {
    expect(readRedirect(state)).toBeUndefined()
  })

  // The open-redirect guard. `//evil.example.com` is a protocol-relative URL: it
  // starts with a slash, so a naive "is it internal?" check passes it, and the
  // browser then navigates off-site. Rejecting it is the entire reason this
  // function validates rather than just reading `.from`.
  it('rejects a protocol-relative URL', () => {
    expect(readRedirect({ from: '//evil.example.com' })).toBeUndefined()
  })

  it('rejects a backslash-prefixed URL that some parsers normalize', () => {
    expect(readRedirect({ from: '\\\\evil.example.com' })).toBeUndefined()
  })

  it.each(['https://evil.example.com', 'javascript:alert(1)', 'prescriptions', ''])(
    'rejects %j because it is not an internal path',
    (from) => {
      expect(readRedirect({ from })).toBeUndefined()
    },
  )

  // A single leading slash followed by another path segment is the shape
  // RequireAuth actually produces, including for nested routes.
  it.each(['/', '/prescriptions/123', '/a/b/c?q=1#frag'])('accepts %j', (from) => {
    expect(readRedirect({ from })).toBe(from)
  })
})
