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

export type RegisterRequest = {
  firstName: string
  lastName: string
  email: string
  password: string
  phone?: string
  address?: Address
}

export type UserResponse = {
  id: string
  firstName: string
  lastName: string
  email: string
  phone?: string
  address?: Address
}

export type Address = {
  street1: string
  street2?: string
  city: string
  state: string
  zip: string
}
