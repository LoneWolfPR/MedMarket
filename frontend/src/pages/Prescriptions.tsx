import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import useAuth from '../auth/useAuth'
import { type ReactElement } from 'react'
import { inputClass, inputFieldGroupClass } from './sharedClasses'
import { type ApiError, type Prescription, type PrescriptionListResponse } from '../api/types'

const MAX_SIZE = 10 * 1024 * 1024
const VALID_TYPES = ['image/png', 'application/pdf', 'image/jpeg']
const VALID_UNITS = ['mg', 'ml', 'mcg'] as const
const schema = z.object({
  physicianName: z.string().min(1, 'Required'),
  medName: z.string().min(1, 'Required'),
  strengthValue: z.string().min(1, 'Required'),
  strengthUnit: z.enum(VALID_UNITS, { message: 'invalid unit' }),
  quantity: z.coerce.number().int().positive(),
  document: z
    .instanceof(FileList)
    .refine((fileList) => fileList.length === 1, 'no file selected')
    .refine((fileList) => {
      if (fileList.length > 0) {
        return fileList[0].size < MAX_SIZE
      }
      return true
    }, 'max upload size is 10Mb')
    .refine((fileList) => {
      if (fileList.length > 0) {
        return VALID_TYPES.includes(fileList[0].type)
      }
      return true
    }, 'invalid file type'),
})

type FormValues = z.infer<typeof schema>

export default function Prescriptions() {
  const { token } = useAuth()
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
    setError,
  } = useForm<FormValues>({ resolver: zodResolver(schema) })

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
        const body: PrescriptionListResponse = await resp.json()
        return body.prescriptions
      } else {
        throw new Error('error fetching prescriptions')
      }
    },
  })

  const uploadMutation = useMutation({
    mutationFn: async (fv: FormValues) => {
      const { document: doc, ...fields } = fv
      const formData = new FormData()
      Object.entries(fields).forEach(([name, value]) => formData.append(name, String(value)))
      formData.append('document', doc[0])
      let resp: Response
      try {
        resp = await fetch('/api/prescriptions/upload', {
          method: 'POST',
          headers: { Authorization: `Bearer ${token}` },
          body: formData,
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
      reset()
    },
  })
  const onSubmit = async (values: FormValues) => {
    try {
      await uploadMutation.mutateAsync(values)
    } catch (e: unknown) {
      if (e instanceof Error) {
        setError('root', { message: e.message })
      } else {
        console.error('unexpected error')
      }
    }
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
        {errors.root && <p className="text-sm text-red-600">{errors.root.message}</p>}
        {uploadMutation.isSuccess && (
          <p className="text-sm text-emerald-600">prescription uploaded</p>
        )}
        <form
          onSubmit={handleSubmit(onSubmit)}
          onChange={uploadMutation.reset}
          noValidate
          className="flex flex-col gap-4"
        >
          <div className={inputFieldGroupClass}>
            <label htmlFor="medName" className="text-sm font-medium text-slate-700">
              Medication Name
            </label>
            <input id="medName" type="text" {...register('medName')} className={inputClass} />
            {errors.medName && <p className="text-sm text-red-600">{errors.medName.message}</p>}
          </div>
          <div className={inputFieldGroupClass}>
            <label htmlFor="physicianName" className="text-sm font-medium text-slate-700">
              Physician Name
            </label>
            <input
              id="physicianName"
              type="text"
              {...register('physicianName')}
              className={inputClass}
            />
            {errors.physicianName && (
              <p className="text-sm text-red-600">{errors.physicianName.message}</p>
            )}
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <div className={inputFieldGroupClass}>
              <label htmlFor="strengthValue" className="text-sm font-medium text-slate-700">
                Strength
              </label>
              <input
                id="strengthValue"
                type="text"
                {...register('strengthValue')}
                className={inputClass}
              />
              {errors.strengthValue && (
                <p className="text-sm text-red-600">{errors.strengthValue.message}</p>
              )}
            </div>
            <div className={inputFieldGroupClass}>
              <label htmlFor="strengthUnit" className="text-sm font-medium text-slate-700">
                Unit
              </label>
              <select id="strengthUnit" {...register('strengthUnit')} className={inputClass}>
                <option value="">Select unit</option>
                {VALID_UNITS.map((unit) => (
                  <option key={unit} value={unit}>
                    {unit}
                  </option>
                ))}
              </select>
              {errors.strengthUnit && (
                <p className="text-sm text-red-600">{errors.strengthUnit.message}</p>
              )}
            </div>
            <div className={inputFieldGroupClass}>
              <label htmlFor="quantity" className="text-sm font-medium text-slate-700">
                Quantity
              </label>
              <input id="quantity" type="number" {...register('quantity')} className={inputClass} />
              {errors.quantity && <p className="text-sm text-red-600">{errors.quantity.message}</p>}
            </div>
          </div>
          <div className={inputFieldGroupClass}>
            <label htmlFor="document" className="text-sm font-medium text-slate-700">
              Document
            </label>
            <input
              type="file"
              id="document"
              {...register('document')}
              accept="application/pdf,image/png,image/jpeg"
              className={`${inputClass}
                 file:px-3 file:py-1.5 file:cursor-pointer file:bg-slate-100 file:hover:bg-slate-200 file:text-slate-700 file:rounded-md file:text-sm file:font-medium`}
            />
            <p className="text-xs text-slate-400">
              Accepts files of type pdf, png, and jpg up to 10MB
            </p>
            {errors.document && <p className="text-sm text-red-600">{errors.document.message}</p>}
          </div>
          <button
            type="submit"
            disabled={isSubmitting}
            className="w-full sm:w-auto bg-teal-600 text-white text-sm font-medium px-4 py-2 rounded-lg cursor-pointer hover:bg-teal-700 focus:ring-teal-600 focus:ring-2 focus:ring-offset-2 focus:outline-hidden disabled:bg-slate-300 disabled:text-slate-500 disabled:cursor-not-allowed"
          >
            {isSubmitting ? 'Uploading...' : 'Upload'}
          </button>
        </form>
      </div>
      {loadState}
    </div>
  )
}
