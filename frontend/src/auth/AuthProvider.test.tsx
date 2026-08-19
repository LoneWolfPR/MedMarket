import { act, render } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AuthProvider from './AuthProvider'
import useAuth from './useAuth'
import type { AuthValue } from './AuthContext'
import { base64url, expiredToken, makeToken, unexpiredToken } from '../test/tokens'

// The code under test reads exactly two things off a Response, so the double
// provides exactly two things. A fuller fake would imply the code depends on
// more of the interface than it does.
function jsonResponse(status: number, body: unknown): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as unknown as Response
}

const fetchMock = vi.fn<typeof fetch>()

// Captured from inside the tree: `login`/`logout` are only reachable through
// the context, and a hook can only be called during a render.
let latest: AuthValue
// One entry per render, so a test can assert what was true on the *first* one
// rather than only on the settled result.
let authFlags: boolean[]

function Probe() {
  const auth = useAuth()
  latest = auth
  authFlags.push(auth.isAuthenticated)
  return null
}

// AuthProvider calls useQueryClient so it can drop cached data when identity
// changes, so it only works beneath a QueryClientProvider. A fresh client per
// render keeps one test's cached entries out of the next.
let queryClient: QueryClient

function renderAuth() {
  queryClient = new QueryClient()
  render(
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <Probe />
      </AuthProvider>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  authFlags = []
  vi.stubGlobal('fetch', fetchMock)
})

describe('AuthProvider', () => {
  it('starts unauthenticated when localStorage holds no token', () => {
    renderAuth()

    expect(latest.token).toBeNull()
    expect(latest.isAuthenticated).toBe(false)
  })

  // The load-bearing property behind reading localStorage in useReducer's lazy
  // initializer instead of an effect: a hard refresh of a protected URL must
  // never render one frame as logged out, or RequireAuth would bounce to /login.
  it('is authenticated on the very first render when a token is stored', () => {
    const token = unexpiredToken()
    localStorage.setItem('token', token)

    renderAuth()

    // One render, and it was already authenticated. The single-entry assertion
    // is what makes this test bite: had the token been read in an effect, the
    // tree would have rendered false and then re-rendered true, so authFlags
    // would read [false, true] and the count would give it away.
    expect(authFlags).toEqual([true])
    expect(latest.token).toBe(token)
  })

  it('stores the token and authenticates on a successful login', async () => {
    fetchMock.mockResolvedValue(jsonResponse(200, { token: 'fresh-token' }))

    renderAuth()
    // Wrapped in act because the resolved login dispatches, and the assertions
    // below read the state that dispatch produces.
    await act(async () => {
      await latest.login('user@example.com', 'correct-horse')
    })

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/auth/login',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ email: 'user@example.com', password: 'correct-horse' }),
      }),
    )
    expect(latest.isAuthenticated).toBe(true)
    expect(localStorage.getItem('token')).toBe('fresh-token')
  })

  // Rejected logins take no act() wrapper: the failure paths dispatch nothing,
  // so there is no state update for React to warn about.
  it('surfaces the server message and stays unauthenticated on a rejected login', async () => {
    fetchMock.mockResolvedValue(jsonResponse(401, { message: 'invalid credentials' }))

    renderAuth()

    await expect(latest.login('user@example.com', 'wrong')).rejects.toThrow('invalid credentials')
    expect(latest.isAuthenticated).toBe(false)
    expect(localStorage.getItem('token')).toBeNull()
  })

  // fetch rejects only on network failure, and the provider translates that into
  // copy a user can read while keeping the original as `cause`.
  it('translates a network failure into human copy and preserves the cause', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const networkFailure = new TypeError('Failed to fetch')
    fetchMock.mockRejectedValue(networkFailure)

    renderAuth()

    await expect(latest.login('user@example.com', 'correct-horse')).rejects.toThrow(
      'Unable to reach the server',
    )
    await expect(latest.login('user@example.com', 'correct-horse')).rejects.toMatchObject({
      cause: networkFailure,
    })
  })

  it('clears the stored token on logout', () => {
    localStorage.setItem('token', unexpiredToken())

    renderAuth()
    act(() => {
      latest.logout()
    })

    expect(latest.isAuthenticated).toBe(false)
    expect(localStorage.getItem('token')).toBeNull()
  })
})

// init() reads `exp` so an expired token never renders a logged-in shell whose
// every request then 401s. This is UX, never security: nothing here verifies a
// signature, and the server stays the only authority on whether a token is good.
describe('AuthProvider stored token validation', () => {
  // Every rejection asserts the same two things — the app boots logged out, and
  // the dead token is gone rather than left to be re-parsed on the next boot.
  function expectRejected() {
    expect(latest.isAuthenticated).toBe(false)
    expect(latest.token).toBeNull()
    expect(localStorage.getItem('token')).toBeNull()
  }

  it('accepts a token whose exp is in the future', () => {
    const token = unexpiredToken()
    localStorage.setItem('token', token)

    renderAuth()

    expect(latest.isAuthenticated).toBe(true)
    // A good token must survive the boot it was read on.
    expect(localStorage.getItem('token')).toBe(token)
  })

  it('rejects a token whose exp has passed', () => {
    localStorage.setItem('token', expiredToken())

    renderAuth()

    expectRejected()
  })

  // The rest of this block is the fail-open family: a payload that does not
  // carry a usable `exp` must be rejected, not waved through because the
  // expiry comparison had nothing to compare.
  it('rejects a token whose payload has no exp', () => {
    localStorage.setItem('token', makeToken({ sub: 'user-1' }))

    renderAuth()

    expectRejected()
  })

  it('rejects a token whose exp is not a number', () => {
    // JSON.parse returns `any`, so the DecodedJwt annotation promises this
    // cannot happen while doing nothing to prevent it.
    localStorage.setItem('token', makeToken({ sub: 'user-1', exp: '99999999999' }))

    renderAuth()

    expectRejected()
  })

  it('rejects a token that is not three segments', () => {
    localStorage.setItem('token', 'not-a-jwt')

    renderAuth()

    expectRejected()
  })

  it('rejects a token whose payload is not decodable', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    // '!' is outside the base64 alphabet, so atob throws rather than returning
    // something JSON.parse could choke on later.
    localStorage.setItem('token', 'aaa.!!!!.bbb')

    renderAuth()

    expectRejected()
  })

  it('rejects a token whose payload decodes to something that is not JSON', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    localStorage.setItem('token', `aaa.${base64url('definitely not json')}.bbb`)

    renderAuth()

    expectRejected()
  })

  // JWTs are base64url, whose last two alphabet characters are '-' and '_'
  // where standard base64 uses '+' and '/'. atob only knows the standard
  // alphabet, so without the substitution it throws on a payload like this one
  // and a perfectly good token reads as garbage. This `sub` is chosen so its
  // encoding contains both characters, which fails if either replace is missing.
  it('accepts a payload containing base64url characters', () => {
    const exp = Math.floor(Date.now() / 1000) + 3600
    localStorage.setItem('token', makeToken({ sub: 'user-a?bc>', exp }))

    renderAuth()

    expect(latest.isAuthenticated).toBe(true)
  })
})

describe('AuthProvider cache isolation', () => {
  // Query keys do not include the user, so nothing else stops one account's cached
  // data from being served to the next one. Clearing on identity change is the guard.
  const cachedList = [{ id: 'rx-1' }]

  it('drops cached data on logout', () => {
    localStorage.setItem('token', unexpiredToken())

    renderAuth()
    queryClient.setQueryData(['prescriptions'], cachedList)
    act(() => {
      latest.logout()
    })

    expect(queryClient.getQueryData(['prescriptions'])).toBeUndefined()
  })

  it('drops cached data on a successful login', async () => {
    // Signing in as a different user without logging out first — reachable by going
    // straight to /login while already authenticated.
    fetchMock.mockResolvedValue(jsonResponse(200, { token: 'second-user-token' }))

    renderAuth()
    queryClient.setQueryData(['prescriptions'], cachedList)
    await act(async () => {
      await latest.login('second@example.com', 'correct-horse')
    })

    expect(queryClient.getQueryData(['prescriptions'])).toBeUndefined()
  })

  it('keeps cached data when a login attempt fails', async () => {
    // A fumbled password must not destroy the still-valid session's data: the user
    // is logged in, mistyped someone else's credentials, and goes back to work.
    localStorage.setItem('token', unexpiredToken())
    fetchMock.mockResolvedValue(jsonResponse(401, { message: 'invalid credentials' }))

    renderAuth()
    queryClient.setQueryData(['prescriptions'], cachedList)

    await expect(latest.login('second@example.com', 'wrong')).rejects.toThrow()
    expect(queryClient.getQueryData(['prescriptions'])).toEqual(cachedList)
  })
})
