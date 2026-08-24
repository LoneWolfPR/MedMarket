import { z } from 'zod'
import { US_STATE_ABBREVIATIONS } from '../api/constants'

const schema = z
  .object({
    firstName: z.string().trim().min(1, 'Required'),
    lastName: z.string().trim().min(1, 'Required'),
    street1: z.string().trim(),
    street2: z.string().trim(),
    city: z.string().trim(),
    state: z.enum(US_STATE_ABBREVIATIONS).or(z.literal('')),
    zip: z.string().regex(/^(\d{5})?$/, 'Zip code must be 5 numbers'),
    email: z.string().email('Enter a valid email'),
    phone: z
      .string()
      .regex(
        /^(\(\d{3}\) ?\d{3}-\d{4}|\d{3}-\d{3}-\d{4}|\d{3}\.\d{3}\.\d{4}|\d{10})?$/,
        'Enter a phone number like 555-123-4567',
      ),
    password: z
      .string()
      .min(8, 'Password must be at least 8 characters')
      .max(32, 'Password cannot be longer than 32 characters')
      .regex(/[A-Z]/, 'Password must contain an upper case character')
      .regex(/[a-z]/, 'Password must contain a lower case character')
      .regex(/[0-9]/, 'Password must contain a number')
      .regex(/[`!@#$%^&*()_+=[\]{}|\\:;"'<,>.?/~-]/, 'Password must contain a special character')
      .regex(
        /^[A-Za-z0-9`!@#$%^&*()_+=[\]{}|\\:;"'<,>.?/~-]+$/,
        'Password contains an invalid character',
      ),
    confirmPassword: z.string(),
  })
  .refine((data) => data.password === data.confirmPassword, {
    message: 'Passwords do not match',
    path: ['confirmPassword'],
  })
  .superRefine((data, ctx) => {
    if (
      data.street1.length > 0 ||
      data.street2.length > 0 ||
      data.city.length > 0 ||
      data.state.length > 0 ||
      data.zip.length > 0
    ) {
      if (data.street1.length === 0) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: 'Street 1 is missing',
          path: ['street1'],
        })
      }
      if (data.city.length === 0) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: 'City is missing',
          path: ['city'],
        })
      }
      if (data.state.length === 0) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: 'State is missing',
          path: ['state'],
        })
      }
      if (data.zip.length === 0) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: 'Zip is missing',
          path: ['zip'],
        })
      }
    }
  })

export type FormValues = z.infer<typeof schema>

export default schema
