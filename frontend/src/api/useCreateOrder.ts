import { useMutation } from '@tanstack/react-query'
import useAuthedApi from './useAuthedApi'
import { type OrderResponse } from './types'

// A real client would get this from Stripe Elements. This is a
// deliberate simplification for this practice application
const PAYMENT_METHOD = 'pm_card_visa'

export default function useCreateOrder() {
  const authedApi = useAuthedApi()
  return useMutation({
    mutationFn: async (offerId: string) => {
      const resp: OrderResponse = await authedApi('/api/orders',
        {
          method: 'POST',
          body: JSON.stringify({
            offerId,
            paymentMethod: PAYMENT_METHOD
          })
        }
      )
      return resp
    }
  })
}
