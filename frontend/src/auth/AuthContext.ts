import { createContext } from 'react'

export type AuthState = {
  token: string | null
}

export type AuthValue = {
  token: string | null
  isAuthenticated: boolean
  login: (email: string, password: string) => Promise<void>
  logout: () => void
}

export const AuthContext = createContext<AuthValue | undefined>(undefined)
