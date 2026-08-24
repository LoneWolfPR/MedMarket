import { useParams } from 'react-router'
import { useQuery } from '@tanstack/react-query'
import { type ReactElement } from 'react'
import useAuthedApi from '../api/useAuthedApi'
import usePrescriptions from '../api/usePrescriptions'
import { type PriceQuoteResponse } from '../api/types'
import { formatCents } from '../api/money'

export default function PriceSearch() {
  const ttl = 15 * 60 * 1000
  const { id } = useParams()
  const authedApi = useAuthedApi()
  const prescriptions = usePrescriptions()
  const searchResults = useQuery({
    queryKey: ['search', id],
    queryFn: async () => {
      const body: PriceQuoteResponse = await authedApi(`/api/prescriptions/${id}/search`, {
        method: 'POST',
      })
      return body.quotes
    },
    refetchOnWindowFocus: false,
    staleTime: ttl,
    gcTime: ttl,
  })

  const prescription = prescriptions.data?.find((rx) => rx.id === id)

  let loadState: ReactElement
  if (searchResults.isPending || prescriptions.isPending) {
    loadState = <p className="text-slate-600">Searching...</p>
  } else if (searchResults.isError || prescriptions.isError || !prescription) {
    loadState = <p className="text-sm text-red-600">Error searching</p>
  } else if (searchResults.data.length === 0) {
    loadState = (
      <div className="bg-white border border-slate-300 border-dashed rounded-lg p-8 text-center">
        <p className="text-slate-600">No matches found</p>
      </div>
    )
  } else {
    loadState = (
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
                {`${formatCents(result.unitPriceCents)} each \u00D7 ${prescription.quantity}`}
              </p>
            </div>
            <div className="flex items-center gap-4">
              <p className="text-lg font-semibold text-slate-900">
                {formatCents(result.totalCents)}
              </p>
              <a
                href={`/`}
                className="bg-teal-600 text-white text-sm font-medium px-4 py-2 rounded-lg hover:bg-teal-700 focus:outline-hidden focus:ring-2 focus:ring-teal-600 focus:ring-offset-2 disabled:bg-slate-300 disabled:text-slate-500 disabled:cursor-not-allowed"
              >
                Order
              </a>
            </div>
          </li>
        ))}
      </ul>
    )
  }
  return (
    <div className="flex flex-col gap-8">
      <h1 className="text-2xl font-semibold tracking-tight text-slate-900">Search Results</h1>
      {loadState}
    </div>
  )
}
