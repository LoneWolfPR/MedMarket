import { act, render } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AuthProvider from './AuthProvider'
import useAuth from './useAuth'
import type { AuthValue } from './AuthContext'

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

function renderAuth() {
  render(
    <AuthProvider>
      <Probe />
    </AuthProvider>,
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
    localStorage.setItem('token', 'stored-token')

    renderAuth()

    // One render, and it was already authenticated. The single-entry assertion
    // is what makes this test bite: had the token been read in an effect, the
    // tree would have rendered false and then re-rendered true, so authFlags
    // would read [false, true] and the count would give it away.
    expect(authFlags).toEqual([true])
    expect(latest.token).toBe('stored-token')
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
    localStorage.setItem('token', 'stored-token')

    renderAuth()
    act(() => {
      latest.logout()
    })

    expect(latest.isAuthenticated).toBe(false)
    expect(localStorage.getItem('token')).toBeNull()
  })
})
