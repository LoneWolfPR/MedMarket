import { useParams } from 'react-router'
import { useState, type ReactElement } from 'react'
import usePrescriptions from '../api/usePrescriptions'
import { type OrderResponse, type PriceQuote } from '../api/types'
import Quotes from '../components/Quotes'

type QuotesStepState = {
  status: 'quotes'
}

type ConfirmStepState = {
  status: 'confirm'
  quote: PriceQuote
}

type PlacedStepState = {
  status: 'placed'
  orderResponse: OrderResponse
}

type StepState = QuotesStepState | ConfirmStepState | PlacedStepState
const defaultState: QuotesStepState = {
  status: 'quotes',
}

export default function PriceSearch() {
  const [step, setStep] = useState<StepState>(defaultState)
  const { id } = useParams()
  const prescriptions = usePrescriptions()
  const prescription = prescriptions.data?.find((rx) => rx.id === id)

  let loadState: ReactElement
  if (prescriptions.isPending) {
    loadState = <p className="text-slate-600">Searching...</p>
  } else if (prescriptions.isError || !prescription) {
    loadState = <p className="text-sm text-red-600">Error searching</p>
  } else {
    switch (step.status) {
      case 'quotes':
        loadState = <Quotes rx={prescription} onSelect={(quote) => {
          setStep({
            status: 'confirm',
            quote
          })
        }} />
        break
      case 'confirm':
        break
      case 'placed':
        break
    }
  }
  return (
    <div className="flex flex-col gap-8">
      <h1 className="text-2xl font-semibold tracking-tight text-slate-900">Search Results</h1>
      {loadState}
    </div>
  )
}
