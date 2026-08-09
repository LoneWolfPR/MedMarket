import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import useAuth from '../auth/useAuth'
import { useRef, useState, type ReactElement } from 'react'
import { inputClass, inputFieldGroupClass } from './sharedClasses'
import { type ApiError, type Prescription } from '../api/types'

type PrescriptionFormValues = {
  physicianName: string
  medName: string
  strengthValue: string
  strengthUnit: string
  quantity: string
}

const EMPTY_FORM: PrescriptionFormValues = {
  physicianName: '',
  medName: '',
  strengthValue: '',
  strengthUnit: '',
  quantity: '',
}

export default function Prescriptions() {
  const { token } = useAuth()
  const [formValues, setFormValues] = useState(EMPTY_FORM)
  const queryClient = useQueryClient()
  const prescriptions = useQuery({
    queryKey: ['prescriptions'],
    queryFn: async () => {
      let resp: Response
      try {
        resp = await fetch('/api/prescriptions', {
          headers: {
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
  const rxDoc = useRef<HTMLInputElement>(null)
  const uploadMutation = useMutation({
    mutationFn: async (fd: FormData) => {
      let resp: Response
      try {
        resp = await fetch('/api/prescriptions/upload', {
          method: 'POST',
          headers: { Authorization: `Bearer ${token}` },
          body: fd,
        })
      } catch (e: unknown) {
        console.error(e)
        throw new Error('error uploading prescription', { cause: e })
      }
      if (resp.ok) {
        return (await resp.json()) as Prescription
      } else {
        const err: ApiError = await resp.json()
        throw new Error(err.message)
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['prescriptions'] })
      setFormValues(EMPTY_FORM)
      if (rxDoc.current) {
        rxDoc.current.value = ''
      }
    },
  })
  const handleSubmit = (e: React.SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    const formData = new FormData()
    Object.entries(formValues).forEach(([name, value]) => formData.append(name, value))
    const file = rxDoc.current?.files?.[0]
    if (!file) {
      return
    }
    formData.append('document', file)
    uploadMutation.mutate(formData)
  }
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
    <div className="flex flex-col gap-8">
      <h1 className="text-2xl font-semibold tracking-tight text-slate-900">Prescriptions</h1>
      <div className="flex flex-col bg-white border border-slate-200 rounded-lg shadow-sm p-6 w-full gap-4">
        <h2 className="text-lg font-semibold text-slate-900">Upload Prescription</h2>
        {uploadMutation.isError && (
          <p className="text-sm text-red-600">{uploadMutation.error.message}</p>
        )}
        {uploadMutation.isSuccess && (
          <p className="text-sm text-emerald-600">prescription uploaded</p>
        )}
        <form
          onSubmit={handleSubmit}
          onChange={uploadMutation.reset}
          className="flex flex-col gap-4"
        >
          <div className={inputFieldGroupClass}>
            <label htmlFor="medName" className="text-sm font-medium text-slate-700">
              Medication Name
            </label>
            <input
              id="medName"
              value={formValues.medName}
              name="medName"
              type="text"
              className={inputClass}
              required
              onChange={(e) =>
                setFormValues((prev) => ({ ...prev, [e.target.name]: e.target.value }))
              }
            />
          </div>
          <div className={inputFieldGroupClass}>
            <label htmlFor="physicianName" className="text-sm font-medium text-slate-700">
              Physician Name
            </label>
            <input
              id="physicianName"
              value={formValues.physicianName}
              name="physicianName"
              type="text"
              className={inputClass}
              required
              onChange={(e) =>
                setFormValues((prev) => ({ ...prev, [e.target.name]: e.target.value }))
              }
            />
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <div className={inputFieldGroupClass}>
              <label htmlFor="strengthValue" className="text-sm font-medium text-slate-700">
                Strength
              </label>
              <input
                id="strengthValue"
                value={formValues.strengthValue}
                name="strengthValue"
                type="text"
                className={inputClass}
                required
                onChange={(e) =>
                  setFormValues((prev) => ({ ...prev, [e.target.name]: e.target.value }))
                }
              />
            </div>
            <div className={inputFieldGroupClass}>
              <label htmlFor="strengthUnit" className="text-sm font-medium text-slate-700">
                Unit
              </label>
              <input
                id="strengthUnit"
                value={formValues.strengthUnit}
                name="strengthUnit"
                type="text"
                className={inputClass}
                required
                onChange={(e) =>
                  setFormValues((prev) => ({ ...prev, [e.target.name]: e.target.value }))
                }
              />
            </div>
            <div className={inputFieldGroupClass}>
              <label htmlFor="quantity" className="text-sm font-medium text-slate-700">
                Quantity
              </label>
              <input
                id="quantity"
                value={formValues.quantity}
                name="quantity"
                type="number"
                className={inputClass}
                required
                onChange={(e) =>
                  setFormValues((prev) => ({ ...prev, [e.target.name]: e.target.value }))
                }
              />
            </div>
          </div>
          <div className={inputFieldGroupClass}>
            <label htmlFor="document" className="text-sm font-medium text-slate-700">
              Document
            </label>
            <input
              type="file"
              id="document"
              required
              ref={rxDoc}
              accept="application/pdf,image/png,image/jpeg"
              className={`${inputClass}
                 file:px-3 file:py-1.5 file:cursor-pointer file:bg-slate-100 file:hover:bg-slate-200 file:text-slate-700 file:rounded-md file:text-sm file:font-medium`}
            />
            <p className="text-xs text-slate-400">
              Accepts files of type pdf, png, and jpg up to 10MB
            </p>
          </div>
          <button
            type="submit"
            disabled={uploadMutation.isPending}
            className="w-full sm:w-auto bg-teal-600 text-white text-sm font-medium px-4 py-2 rounded-lg cursor-pointer hover:bg-teal-700 focus:ring-teal-600 focus:ring-2 focus:ring-offset-2 focus:outline-hidden disabled:bg-slate-300 disabled:text-slate-500 disabled:cursor-not-allowed"
          >
            {uploadMutation.isPending ? 'Uploading...' : 'Upload'}
          </button>
        </form>
      </div>
      {loadState}
    </div>
  )
}
