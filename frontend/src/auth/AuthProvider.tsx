import { useCallback, useMemo, useReducer, type ReactNode } from 'react'
import { AuthContext, type AuthState, type AuthValue } from './AuthContext'
import { type ApiError, type TokenResponse } from '../api/types'

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

function init(): AuthState {
  return { token: localStorage.getItem(LS_TOKEN_KEY) }
}

function AuthProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(authReducer, undefined, init)
  const login = useCallback(async (email: string, password: string) => {
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
    } else {
      const err: ApiError = await resp.json()
      throw new Error(err.message)
    }
  }, [])
  const logout = useCallback(() => {
    localStorage.removeItem(LS_TOKEN_KEY)
    dispatch({ type: 'logged out' })
  }, [])
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
