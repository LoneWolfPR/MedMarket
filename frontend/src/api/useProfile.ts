import { useQuery } from '@tanstack/react-query'
import useAuthedApi from './useAuthedApi'
import { type UserResponse } from './types'

export default function useProfile() {
  const authedApi = useAuthedApi()
  return useQuery({
    queryKey: ['userProfile'],
    queryFn: async () => {
      const body: UserResponse = await authedApi('/api/auth/profile')
      return body
    },
  })
}
