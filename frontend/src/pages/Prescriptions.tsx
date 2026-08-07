import { useQuery } from '@tanstack/react-query'
import useAuth from '../auth/useAuth'
import type { ReactElement } from 'react'

export type Prescription = {
  id: string
  physicianName: string
  medName: string
  strengthValue: string
  strengthUnit: string
  quantity: number
  documentUrl: string
}

export default function Prescriptions() {
  const { token } = useAuth()
  const prescriptions = useQuery({
    queryKey: ['prescriptions'],
    queryFn: async () => {
      let resp: Response
      try {
        resp = await fetch('/api/prescriptions', {
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${token}`,
          },
        })
      } catch (e: unknown) {
        console.error(e)
        throw new Error('error fetching prescriptions', { cause: e })
      }
      if (resp.ok) {
        const rxRecords: Prescription[] = await resp.json()
        return rxRecords
      } else {
        throw new Error('error fetching prescriptions')
      }
    },
  })

  let loadState: ReactElement
  if (prescriptions.isPending) {
    loadState = <p className="text-slate-600">Loading prescriptions...</p>
  } else if (prescriptions.isError) {
    loadState = <p className="text-sm text-red-600">Error loading prescriptions</p>
  } else if (prescriptions.data.length === 0) {
    loadState = (
      <div className="bg-white border border-slate-300 border-dashed rounded-lg p-8 text-center">
        <p className="text-slate-600 text-center">No prescriptions found</p>
        <p className="text-xs text-slate-400">Upload prescription</p>
      </div>
    )
  } else {
    loadState = (
      <ul className="flex flex-col gap-4">
        {prescriptions.data.map((rx) => (
          <li
            key={rx.id}
            className="flex flex-col sm:flex-row sm:justify-between sm:items-center bg-white border border-slate-200 rounded-lg shadow-sm p-4 gap-3"
          >
            <div className="flex flex-col gap-1">
              <p className="text-base font-medium text-slate-900">
                {rx.medName} {rx.strengthValue}
                {rx.strengthUnit}
              </p>
              <p className="text-sm text-slate-600">Prescribed by: {rx.physicianName}</p>
              <p className="text-xs text-slate-400">Qty: {rx.quantity}</p>
            </div>
            <a
              href={rx.documentUrl}
              rel="noopener noreferrer"
              target="_blank"
              className="text-sm font-medium text-teal-600 hover:text-teal-700 hover:underline"
            >
              View document
            </a>
          </li>
        ))}
      </ul>
    )
  }
  return (
    <div>
      <h1 className="text-2xl font-semibold tracking-tight text-slate-900">Prescriptions</h1>
      {loadState}
    </div>
  )
}
