import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router'
import { describe, expect, it } from 'vitest'
import Layout from './Layout'
import AuthProvider from '../auth/AuthProvider'
import { unexpiredToken } from '../test/tokens'

// Layout reads auth from context and renders router primitives, so it needs
// both providers. Seeding localStorage first is how a real reload authenticates
// — and AuthProvider now decodes what it finds there, so the seed has to be a
// real unexpired JWT rather than any placeholder string.
function renderLayout({ token }: { token?: string } = {}) {
  if (token) {
    localStorage.setItem('token', token)
  }
  return render(
    <MemoryRouter>
      <QueryClientProvider client={new QueryClient()}>
        <AuthProvider>
          <Layout />
        </AuthProvider>
      </QueryClientProvider>
    </MemoryRouter>,
  )
}

describe('Layout', () => {
  it('offers only the login link when logged out', () => {
    renderLayout()

    expect(screen.getByRole('link', { name: 'Login' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Prescriptions' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Logout' })).not.toBeInTheDocument()
  })

  it('offers the authenticated nav when a token is present', () => {
    renderLayout({ token: unexpiredToken() })

    expect(screen.getByRole('link', { name: 'Prescriptions' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Logout' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Login' })).not.toBeInTheDocument()
  })

  it('returns to the logged-out nav after logging out', async () => {
    const user = userEvent.setup()
    renderLayout({ token: unexpiredToken() })

    await user.click(screen.getByRole('button', { name: 'Logout' }))

    expect(screen.getByRole('link', { name: 'Login' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Logout' })).not.toBeInTheDocument()
    expect(localStorage.getItem('token')).toBeNull()
  })
})
