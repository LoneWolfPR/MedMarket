import { useQuery } from '@tanstack/react-query'
import useAuthedApi from './useAuthedApi'
import { type PrescriptionListResponse } from './types'

export default function usePrescriptions() {
  const authedApi = useAuthedApi()
  return useQuery({
    queryKey: ['prescriptions'],
    queryFn: async () => {
      const body: PrescriptionListResponse = await authedApi('/api/prescriptions')
      return body.prescriptions
    },
  })
}
