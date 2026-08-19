// The app only ever *decodes* a token — it has no secret and never verifies a
// signature — so a test fixture needs a real `header.payload.signature` shape
// with a genuinely base64url-encoded payload. The signature segment is read by
// nothing, so a placeholder is honest here rather than lazy.

export function base64url(value: string): string {
  return btoa(value).replaceAll('+', '-').replaceAll('/', '_').replace(/=+$/, '')
}

const header = base64url(JSON.stringify({ alg: 'HS256', typ: 'JWT' }))

export function makeToken(payload: Record<string, unknown>): string {
  return `${header}.${base64url(JSON.stringify(payload))}.signature-not-checked-here`
}

// `exp` is seconds since the epoch (RFC 7519 NumericDate), which is what the Go
// issuer emits via jwt.NewNumericDate — not the milliseconds Date.now() returns.
function epochSecondsFromNow(offsetSeconds: number): number {
  return Math.floor(Date.now() / 1000) + offsetSeconds
}

export function unexpiredToken(): string {
  return makeToken({ sub: 'user-1', exp: epochSecondsFromNow(3600) })
}

export function expiredToken(): string {
  return makeToken({ sub: 'user-1', exp: epochSecondsFromNow(-60) })
}
