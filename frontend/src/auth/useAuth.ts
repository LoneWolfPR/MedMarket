import { useContext } from 'react'
import { AuthContext, type AuthValue } from './AuthContext'

export default function useAuth(): AuthValue {
  const value = useContext(AuthContext)
  if (value == undefined) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return value
}
