import { useCallback } from 'react'
import useAuth from '../auth/useAuth'
import { type ApiError } from './types'
import ApiResponseError from './ApiResponseError'

export default function useAuthedApi() {
  const { token, logout } = useAuth()
  return useCallback(
    async <T>(path: string, init?: RequestInit): Promise<T> => {
      let resp: Response
      try {
        const opts = { ...init }
        opts.headers = { ...opts.headers, Authorization: `Bearer ${token}` }
        resp = await fetch(path, opts)
      } catch (e: unknown) {
        console.error(e)
        throw new Error('error making request', { cause: e })
      }
      if (resp.ok) {
        try {
          const data = await resp.json()
          return data
        } catch (e: unknown) {
          console.error(e)
          throw new Error('error making request', { cause: e })
        }
      } else {
        if (resp.status === 401) {
          logout()
          throw new ApiResponseError('unauthorized', 401)
        }
        let error: ApiError
        try {
          error = await resp.json()
        } catch (e: unknown) {
          console.error(e)
          throw new Error('error making request', { cause: e })
        }
        if (error.message) {
          throw new ApiResponseError(error.message, resp.status)
        }
        throw new ApiResponseError('error making request', resp.status)
      }
    },
    [token, logout],
  )
}
