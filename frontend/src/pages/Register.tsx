import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Link, useLocation, useNavigate } from 'react-router'
import { inputClass, inputFieldGroupClass } from './sharedClasses'
import { type RegisterRequest, type ApiError, type UserResponse } from '../api/types'
import { US_STATE_ABBREVIATIONS } from '../api/constants'
import useAuth from '../auth/useAuth'
import { readRedirect } from '../auth/locationState'
import schema, { type FormValues } from './registerSchema'

async function registerUser(fv: FormValues) {
  const regBody: RegisterRequest = {
    firstName: fv.firstName,
    lastName: fv.lastName,
    email: fv.email,
    ...(fv.phone && { phone: fv.phone }),
    password: fv.password,
    ...(fv.street1 && {
      address: {
        street1: fv.street1,
        ...(fv.street2 && { street2: fv.street2 }),
        city: fv.city,
        state: fv.state,
        zip: fv.zip,
      },
    }),
  }
  let resp: Response
  try {
    resp = await fetch('/api/auth/register', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(regBody),
    })
  } catch (e: unknown) {
    console.error(e)
    throw new Error('Unable to reach the server', { cause: e })
  }
  if (resp.ok) {
    const newUser: UserResponse = await resp.json()
    return newUser
  }
  let err: ApiError
  try {
    err = await resp.json()
  } catch (e: unknown) {
    throw new Error('error registering new user', { cause: e })
  }
  if (err.message) {
    throw new Error(err.message)
  }
  throw new Error('error registering new user')
}

function Register() {
  const navigate = useNavigate()
  const { login } = useAuth()
  const from = readRedirect(useLocation().state)

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
    setError,
  } = useForm<FormValues>({ resolver: zodResolver(schema) })

  const onSubmit = async (values: FormValues) => {
    try {
      await registerUser(values)
    } catch (e: unknown) {
      if (e instanceof Error) {
        setError('root', { message: e.message })
      } else {
        setError('root', { message: 'Something went wrong' })
        console.error('An unexpected error occurred: ', e)
      }
      return
    }
    try {
      await login(values.email, values.password)
    } catch (e: unknown) {
      console.error('error logging in', e)
      navigate('/login', { replace: true, state: { from, registered: true } })
      return
    }

    navigate(from ?? '/', { replace: true })
  }

  return (
    <div className="bg-white border border-slate-200 rounded-lg shadow-sm p-6 max-w-2xl mx-auto flex flex-col gap-4">
      <div className="flex flex-col gap-1">
        <h1 className="text-2xl font-semibold tracking-tight text-slate-900">Register</h1>
        <p className="text-xs text-slate-500">
          <span aria-hidden="true">*</span> Required
        </p>
      </div>
      <form onSubmit={handleSubmit(onSubmit)} noValidate className="flex flex-col gap-4">
        {errors.root && (
          <p role="alert" className="text-sm text-red-600">
            {errors.root.message}
          </p>
        )}
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div className={inputFieldGroupClass}>
            <label htmlFor="firstName" className="text-sm font-medium text-slate-700">
              First Name{' '}
              <span aria-hidden="true" className="text-slate-500">
                *
              </span>
            </label>
            <input
              id="firstName"
              autoComplete="given-name"
              type="text"
              required
              aria-invalid={!!errors.firstName}
              aria-describedby={errors.firstName ? 'firstName-error' : undefined}
              {...register('firstName')}
              className={inputClass}
            />
            {errors.firstName && (
              <p id="firstName-error" className="text-xs text-red-600">
                {errors.firstName.message}
              </p>
            )}
          </div>
          <div className={inputFieldGroupClass}>
            <label htmlFor="lastName" className="text-sm font-medium text-slate-700">
              Last Name{' '}
              <span aria-hidden="true" className="text-slate-500">
                *
              </span>
            </label>
            <input
              id="lastName"
              autoComplete="family-name"
              type="text"
              required
              aria-invalid={!!errors.lastName}
              aria-describedby={errors.lastName ? 'lastName-error' : undefined}
              {...register('lastName')}
              className={inputClass}
            />
            {errors.lastName && (
              <p id="lastName-error" className="text-xs text-red-600">
                {errors.lastName.message}
              </p>
            )}
          </div>
          <div className={inputFieldGroupClass}>
            <label htmlFor="phone" className="text-sm font-medium text-slate-700">
              Phone
            </label>
            <input
              id="phone"
              autoComplete="tel"
              type="text"
              aria-invalid={!!errors.phone}
              aria-describedby={errors.phone ? 'phone-error' : undefined}
              {...register('phone')}
              className={inputClass}
            />
            {errors.phone && (
              <p id="phone-error" className="text-xs text-red-600">
                {errors.phone.message}
              </p>
            )}
          </div>
          <div className={inputFieldGroupClass}>
            <label htmlFor="email" className="text-sm font-medium text-slate-700">
              Email{' '}
              <span aria-hidden="true" className="text-slate-500">
                *
              </span>
            </label>
            <input
              id="email"
              type="email"
              autoComplete="email"
              required
              aria-invalid={!!errors.email}
              aria-describedby={errors.email ? 'email-error' : undefined}
              {...register('email')}
              className={inputClass}
            />
            {errors.email && (
              <p id="email-error" className="text-xs text-red-600">
                {errors.email.message}
              </p>
            )}
          </div>
        </div>
        <div className="flex flex-col gap-1.5">
          <p id="password-hint" className="text-xs text-slate-400">
            8&ndash;32 characters with an uppercase letter, a lowercase letter, a number, and a
            symbol. No spaces.
          </p>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div className={inputFieldGroupClass}>
              <label htmlFor="password" className="text-sm font-medium text-slate-700">
                Password{' '}
                <span aria-hidden="true" className="text-slate-500">
                  *
                </span>
              </label>
              <input
                id="password"
                autoComplete="new-password"
                type="password"
                required
                aria-invalid={!!errors.password}
                aria-describedby={
                  errors.password ? 'password-hint password-error' : 'password-hint'
                }
                {...register('password')}
                className={inputClass}
              />
              {errors.password && (
                <p id="password-error" className="text-xs text-red-600">
                  {errors.password.message}
                </p>
              )}
            </div>
            <div className={inputFieldGroupClass}>
              <label htmlFor="confirmPassword" className="text-sm font-medium text-slate-700">
                Confirm Password{' '}
                <span aria-hidden="true" className="text-slate-500">
                  *
                </span>
              </label>
              <input
                id="confirmPassword"
                type="password"
                autoComplete="new-password"
                required
                aria-invalid={!!errors.confirmPassword}
                aria-describedby={
                  errors.confirmPassword ? 'password-hint confirmPassword-error' : 'password-hint'
                }
                {...register('confirmPassword')}
                className={inputClass}
              />
              {errors.confirmPassword && (
                <p id="confirmPassword-error" className="text-xs text-red-600">
                  {errors.confirmPassword.message}
                </p>
              )}
            </div>
          </div>
        </div>
        <div className="flex flex-col gap-1.5">
          <p className="text-xs text-slate-400">
            Address is optional, but if you enter one, everything except Street 2 is required.
          </p>
          <div className="flex flex-col gap-4">
            <div className={inputFieldGroupClass}>
              <label htmlFor="street1" className="text-sm font-medium text-slate-700">
                Street 1
              </label>
              <input
                id="street1"
                autoComplete="address-line1"
                type="text"
                aria-invalid={!!errors.street1}
                aria-describedby={errors.street1 ? 'street1-error' : undefined}
                {...register('street1')}
                className={inputClass}
              />
              {errors.street1 && (
                <p id="street1-error" className="text-xs text-red-600">
                  {errors.street1.message}
                </p>
              )}
            </div>
            <div className={inputFieldGroupClass}>
              <label htmlFor="street2" className="text-sm font-medium text-slate-700">
                Street 2
              </label>
              <input
                id="street2"
                autoComplete="address-line2"
                type="text"
                aria-invalid={!!errors.street2}
                aria-describedby={errors.street2 ? 'street2-error' : undefined}
                {...register('street2')}
                className={inputClass}
              />
              {errors.street2 && (
                <p id="street2-error" className="text-xs text-red-600">
                  {errors.street2.message}
                </p>
              )}
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
              <div className={inputFieldGroupClass}>
                <label htmlFor="city" className="text-sm font-medium text-slate-700">
                  City
                </label>
                <input
                  id="city"
                  autoComplete="address-level2"
                  type="text"
                  aria-invalid={!!errors.city}
                  aria-describedby={errors.city ? 'city-error' : undefined}
                  {...register('city')}
                  className={inputClass}
                />
                {errors.city && (
                  <p id="city-error" className="text-xs text-red-600">
                    {errors.city.message}
                  </p>
                )}
              </div>
              <div className={inputFieldGroupClass}>
                <label htmlFor="state" className="text-sm font-medium text-slate-700">
                  State
                </label>
                <select
                  id="state"
                  autoComplete="address-level1"
                  aria-invalid={!!errors.state}
                  aria-describedby={errors.state ? 'state-error' : undefined}
                  {...register('state')}
                  className={inputClass}
                >
                  <option value="">Select State</option>
                  {US_STATE_ABBREVIATIONS.map((state) => (
                    <option key={state} value={state}>
                      {state}
                    </option>
                  ))}
                </select>
                {errors.state && (
                  <p id="state-error" className="text-xs text-red-600">
                    {errors.state.message}
                  </p>
                )}
              </div>
              <div className={inputFieldGroupClass}>
                <label htmlFor="zip" className="text-sm font-medium text-slate-700">
                  Zip
                </label>
                <input
                  id="zip"
                  type="text"
                  inputMode="numeric"
                  autoComplete="postal-code"
                  aria-invalid={!!errors.zip}
                  aria-describedby={errors.zip ? 'zip-error' : undefined}
                  {...register('zip')}
                  className={inputClass}
                />
                {errors.zip && (
                  <p id="zip-error" className="text-xs text-red-600">
                    {errors.zip.message}
                  </p>
                )}
              </div>
            </div>
          </div>
        </div>

        <button
          type="submit"
          disabled={isSubmitting}
          className="w-full sm:w-auto bg-teal-600 text-white text-sm font-medium px-4 py-2 rounded-lg cursor-pointer hover:bg-teal-700 focus:ring-teal-600 focus:ring-2 focus:ring-offset-2 focus:outline-hidden disabled:bg-slate-300 disabled:text-slate-500 disabled:cursor-not-allowed"
        >
          {isSubmitting ? 'Registering...' : 'Register'}
        </button>
        <p className="text-sm text-slate-600">
          Already have an account?{' '}
          <Link
            to="/login"
            state={{ from }}
            className="font-medium text-teal-600 hover:text-teal-700 hover:underline"
          >
            Login
          </Link>
        </p>
      </form>
    </div>
  )
}

export default Register
