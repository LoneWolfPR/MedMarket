import { useCallback, useMemo, useReducer, type ReactNode } from 'react'
import { AuthContext, type AuthState, type AuthValue } from './AuthContext'
import { type ApiError, type TokenResponse } from '../api/types'
import { useQueryClient } from '@tanstack/react-query'

type AuthAction = { type: 'logged in'; token: string } | { type: 'logged out' }

const LS_TOKEN_KEY = 'token'

function authReducer(_state: AuthState, action: AuthAction): AuthState {
  switch (action.type) {
    case 'logged in':
      return { token: action.token }
    case 'logged out':
      return { token: null }
    default:
      throw new Error('unrecognized auth action')
  }
}

type DecodedJwt = {
  sub: string
  exp: number
}

function init(): AuthState {
  // This token will be in 3 parts separated by a '.' The payload is in the
  // second part
  const token = localStorage.getItem(LS_TOKEN_KEY)
  const tokenParts = token?.split('.')
  if (tokenParts?.length === 3) {
    try {
      const encodedPayload = tokenParts[1].replaceAll('-', '+').replaceAll('_', '/')
      const decoded = atob(encodedPayload)
      const payload: DecodedJwt = JSON.parse(decoded)
      if (typeof payload.exp === 'number' && payload.exp * 1000 > Date.now()) {
        return { token }
      }
    } catch (e: unknown) {
      if (e instanceof Error) {
        console.error(e.message)
      } else {
        console.error('error with token')
      }
    }
  }
  localStorage.removeItem(LS_TOKEN_KEY)
  return { token: null }
}

function AuthProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(authReducer, undefined, init)
  const queryClient = useQueryClient()
  const login = useCallback(
    async (email: string, password: string) => {
      let resp: Response
      try {
        resp = await fetch('/api/auth/login', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({ email, password }),
        })
      } catch (e: unknown) {
        console.error(e)
        throw new Error('Unable to reach the server', { cause: e })
      }

      if (resp.ok) {
        const data: TokenResponse = await resp.json()
        localStorage.setItem(LS_TOKEN_KEY, data.token)
        dispatch({ type: 'logged in', token: data.token })
        queryClient.clear()
      } else {
        let err: ApiError
        try {
          err = await resp.json()
        } catch (e: unknown) {
          throw new Error('error logging in', { cause: e })
        }
        if (err.message) {
          throw new Error(err.message)
        }
        throw new Error('error logging in')
      }
    },
    [queryClient],
  )
  const logout = useCallback(() => {
    localStorage.removeItem(LS_TOKEN_KEY)
    dispatch({ type: 'logged out' })
    queryClient.clear()
  }, [queryClient])
  const value: AuthValue = useMemo(() => {
    return {
      token: state.token,
      isAuthenticated: state.token != null,
      login,
      logout,
    }
  }, [state.token, login, logout])
  return <AuthContext value={value}>{children}</AuthContext>
}

export default AuthProvider
