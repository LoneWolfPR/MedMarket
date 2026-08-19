export type Prescription = {
  id: string
  physicianName: string
  medName: string
  strengthValue: string
  strengthUnit: string
  quantity: number
  documentUrl: string
}

export type TokenResponse = { token: string }

export type ApiError = { message: string }
