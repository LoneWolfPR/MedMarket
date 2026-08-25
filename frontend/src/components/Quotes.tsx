import { useQuery } from '@tanstack/react-query'
import useAuthedApi from '../api/useAuthedApi'
import { type PriceQuote, type Prescription, type PriceQuoteResponse } from '../api/types'
import { ReactElement } from 'react'
import { formatCents } from '../api/money'

type QuotesProps = {
  rx: Prescription
  onSelect: (quote: PriceQuote) => void
}
export default function Quotes({ rx, onSelect}: QuotesProps) {
  const ttl = 15 * 60 * 1000
  const authedApi = useAuthedApi()
  const searchResults = useQuery({
    queryKey: ['search', rx.id],
    queryFn: async () => {
      const body: PriceQuoteResponse = await authedApi(`/api/prescriptions/${rx.id}/search`, {
        method: 'POST',
      })
      return body.quotes
    },
    refetchOnWindowFocus: false,
    staleTime: ttl,
    gcTime: ttl,
  })

  let searchState: ReactElement
  if (searchResults.isPending) {
    searchState = <p className="text-slate-600">Searching...</p>
  } else if (searchResults.isError) {
    searchState = <p className="text-sm text-red-600">Error searching</p>
  } else if (searchResults.data.length === 0) {
    searchState = (
      <div className="bg-white border border-slate-300 border-dashed rounded-lg p-8 text-center">
        <p className="text-slate-600">No matches found</p>
      </div>
    )
  } else {
    searchState = (
      <ul className="flex flex-col gap-4">
        {searchResults.data.map((result, index) => (
          <li
            key={result.offerId}
            className="flex flex-col sm:flex-row sm:justify-between sm:items-center bg-white border border-slate-200 rounded-lg shadow-sm p-4 gap-3"
          >
            <div className="flex flex-col gap-1">
              <div className="flex items-center gap-2">
                <p className="text-base font-medium text-slate-900">
                  {result.pharmacyName.toUpperCase()}
                </p>
                {index === 0 && (
                  <div className="inline-flex items-center rounded-full px-2 py-0.5 bg-emerald-50 text-emerald-700 text-xs font-medium">
                    Best Price
                  </div>
                )}
              </div>
              <p className="text-sm text-slate-600">
                {`${formatCents(result.unitPriceCents)} each \u00D7 ${rx.quantity}`}
              </p>
            </div>
            <div className="flex items-center gap-4">
              <p className="text-lg font-semibold text-slate-900">
                {formatCents(result.totalCents)}
              </p>
              <button
                onClick={() => { onSelect(result)}}
                className="bg-teal-600 text-white text-sm font-medium px-4 py-2 rounded-lg hover:bg-teal-700 focus:outline-hidden focus:ring-2 focus:ring-teal-600 focus:ring-offset-2 disabled:bg-slate-300 disabled:text-slate-500 disabled:cursor-not-allowed"
              >
                Order
              </button>
            </div>
          </li>
        ))}
      </ul>
    )
  }
  return searchState
}
