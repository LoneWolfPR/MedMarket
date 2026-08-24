import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import Register from './Register'
import AuthProvider from '../auth/AuthProvider'
import { unexpiredToken } from '../test/tokens'

function jsonResponse(status: number, body: unknown): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as unknown as Response
}

const fetchMock = vi.fn<typeof fetch>()

// Routes the two calls the flow makes by URL, so a test only has to say what
// each endpoint should do rather than sequence the calls by hand.
function stubEndpoints({ register, login }: { register: Response; login?: Response }) {
  fetchMock.mockImplementation(async (input) => {
    const url = String(input)
    if (url.endsWith('/register')) return register
    if (url.endsWith('/login')) {
      if (!login) throw new Error('login was called but no login response was stubbed')
      return login
    }
    throw new Error(`unexpected fetch: ${url}`)
  })
}

// Renders wherever the flow navigated to, plus the state it carried. Asserting on
// this rather than on a spied navigate() checks the outcome the user experiences.
function Landing({ name }: { name: string }) {
  const { pathname, state } = useLocation()
  return (
    <div>
      <p>{`landed:${name}`}</p>
      <p>{`path:${pathname}`}</p>
      <p>{`state:${JSON.stringify(state)}`}</p>
    </div>
  )
}

function renderRegister(entry: string | { pathname: string; state: unknown } = '/register') {
  // AuthProvider reaches for a query client, so the provider order here mirrors
  // main.tsx: cache outside auth.
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[entry]}>
        <AuthProvider>
          <Routes>
            <Route path="/register" element={<Register />} />
            <Route path="/login" element={<Landing name="login" />} />
            <Route path="/" element={<Landing name="home" />} />
            <Route path="/prescriptions" element={<Landing name="prescriptions" />} />
          </Routes>
        </AuthProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

// The minimum that satisfies the schema: the address is optional as a group, so
// a valid submission can leave all of it blank.
async function fillRequiredFields(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText(/first name/i), 'Ada')
  await user.type(screen.getByLabelText(/last name/i), 'Lovelace')
  await user.type(screen.getByLabelText(/email/i), 'ada@example.com')
  await user.type(screen.getByLabelText(/^password/i), 'Password1!')
  await user.type(screen.getByLabelText(/confirm password/i), 'Password1!')
}

async function submit(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('button', { name: /register/i }))
}

describe('Register', () => {
  beforeEach(() => {
    // `restoreMocks` in vite.config.ts only reaches vi.spyOn spies, so a
    // module-level vi.fn() carries its call log into the next test unless it is
    // reset by hand. The tests below count calls, so that would go unnoticed as
    // a pass rather than a failure.
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
  })

  it('shows the server message and stays put when registration fails', async () => {
    const user = userEvent.setup()
    stubEndpoints({ register: jsonResponse(409, { message: 'Email already registered' }) })
    renderRegister()

    await fillRequiredFields(user)
    await submit(user)

    expect(await screen.findByRole('alert')).toHaveTextContent('Email already registered')
    // Still on the form: a failed registration must not navigate, or the message
    // it just set would never be seen.
    expect(screen.queryByText(/^landed:/)).not.toBeInTheDocument()
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('does not attempt a login when registration fails', async () => {
    const user = userEvent.setup()
    stubEndpoints({ register: jsonResponse(400, { message: 'Bad request' }) })
    renderRegister()

    await fillRequiredFields(user)
    await submit(user)

    await screen.findByRole('alert')
    const calledUrls = fetchMock.mock.calls.map(([input]) => String(input))
    expect(calledUrls).toEqual([expect.stringContaining('/register')])
  })

  it('signs in and lands on the default destination when both calls succeed', async () => {
    const user = userEvent.setup()
    stubEndpoints({
      register: jsonResponse(201, { id: 'u1' }),
      login: jsonResponse(200, { token: unexpiredToken() }),
    })
    renderRegister()

    await fillRequiredFields(user)
    await submit(user)

    expect(await screen.findByText('landed:home')).toBeInTheDocument()
  })

  it('returns to the pending destination when one was carried in', async () => {
    const user = userEvent.setup()
    stubEndpoints({
      register: jsonResponse(201, { id: 'u1' }),
      login: jsonResponse(200, { token: unexpiredToken() }),
    })
    renderRegister({ pathname: '/register', state: { from: '/prescriptions' } })

    await fillRequiredFields(user)
    await submit(user)

    expect(await screen.findByText('landed:prescriptions')).toBeInTheDocument()
  })

  // The account exists but the session does not, so the form is the wrong place
  // to leave the user — resubmitting it would now 409.
  it('sends the user to sign in when registration succeeds but login fails', async () => {
    const user = userEvent.setup()
    stubEndpoints({
      register: jsonResponse(201, { id: 'u1' }),
      login: jsonResponse(401, { message: 'Invalid credentials' }),
    })
    renderRegister()

    await fillRequiredFields(user)
    await submit(user)

    expect(await screen.findByText('landed:login')).toBeInTheDocument()
    expect(screen.getByText(/^state:/)).toHaveTextContent('"registered":true')
  })

  it('forwards the pending destination through a failed login', async () => {
    const user = userEvent.setup()
    stubEndpoints({
      register: jsonResponse(201, { id: 'u1' }),
      login: jsonResponse(401, { message: 'Invalid credentials' }),
    })
    renderRegister({ pathname: '/register', state: { from: '/prescriptions' } })

    await fillRequiredFields(user)
    await submit(user)

    await screen.findByText('landed:login')
    expect(screen.getByText(/^state:/)).toHaveTextContent('"from":"/prescriptions"')
  })

  it('never reaches the network when the form itself is invalid', async () => {
    const user = userEvent.setup()
    stubEndpoints({ register: jsonResponse(201, { id: 'u1' }) })
    renderRegister()

    await user.type(screen.getByLabelText(/first name/i), 'Ada')
    await submit(user)

    expect(await screen.findByText('Enter a valid email')).toBeInTheDocument()
    expect(fetchMock).not.toHaveBeenCalled()
  })
})
