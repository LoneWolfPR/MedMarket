export type Prescription = {
  id: string
  physicianName: string
  medName: string
  strengthValue: string
  strengthUnit: string
  quantity: number
  documentUrl: string
}

// Collection endpoints return an object wrapping the array, not a bare array,
// so the response has room to grow (counts, paging, partial-result warnings)
// without breaking existing clients.
export type PrescriptionListResponse = { prescriptions: Prescription[] }

export type TokenResponse = { token: string }

export type ApiError = { message?: string }
