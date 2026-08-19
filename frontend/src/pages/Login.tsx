import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import useAuth from '../auth/useAuth'
import { useLocation, useNavigate } from 'react-router'
import { inputClass } from './sharedClasses'
import { useMutation } from '@tanstack/react-query'

function isValidRedirect(state: unknown): state is { from: string } {
  return (
    typeof state === 'object' &&
    state !== null &&
    'from' in state &&
    typeof state.from === 'string' &&
    state.from.startsWith('/') &&
    !state.from.startsWith('//')
  )
}

const schema = z.object({
  email: z.string().email('Enter a valid email'),
  password: z.string().min(1, 'Required'),
})

type FormValues = z.infer<typeof schema>

function Login() {
  const navigate = useNavigate()
  const { state: locState } = useLocation()
  const { login } = useAuth()

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
    setError,
  } = useForm<FormValues>({ resolver: zodResolver(schema) })

  const loginMutation = useMutation({
    mutationFn: async ({ email, password }: FormValues) => {
      return login(email, password)
    },
  })

  const onSubmit = async (values: FormValues) => {
    try {
      await loginMutation.mutateAsync(values)
      const dest = isValidRedirect(locState) ? locState.from : '/'
      navigate(dest, { replace: true })
    } catch (e: unknown) {
      if (e instanceof Error) {
        setError('root', { message: e.message })
      } else {
        setError('root', { message: 'Something went wrong' })
        console.error('An unexpected error occurred: ', e)
      }
    }
  }

  return (
    <div className="bg-white border border-slate-200 rounded-lg shadow-sm p-6 max-w-sm mx-auto">
      <h1 className="text-2xl font-semibold tracking-tight text-slate-900">Login</h1>
      <form onSubmit={handleSubmit(onSubmit)} noValidate className="flex flex-col gap-4">
        {errors.root && <p className="text-sm text-red-600">{errors.root.message}</p>}
        <div className="flex flex-col gap-1.5">
          <label htmlFor="email" className="text-sm font-medium text-slate-700">
            Email
          </label>
          <input
            id="email"
            type="email"
            autoComplete="email"
            {...register('email')}
            className={inputClass}
          />
          {errors.email && <p className="text-sm text-red-600">{errors.email.message}</p>}
        </div>
        <div className="flex flex-col gap-1.5">
          <label htmlFor="password" className="text-sm font-medium text-slate-700">
            Password
          </label>
          <input
            id="password"
            type="password"
            autoComplete="current-password"
            {...register('password')}
            className={inputClass}
          />
          {errors.password && <p className="text-sm text-red-600">{errors.password.message}</p>}
        </div>
        <button
          type="submit"
          disabled={isSubmitting}
          className="w-full sm:w-auto bg-teal-600 text-white text-sm font-medium px-4 py-2 rounded-lg cursor-pointer hover:bg-teal-700 focus:ring-teal-600 focus:ring-2 focus:ring-offset-2 focus:outline-hidden disabled:bg-slate-300 disabled:text-slate-500 disabled:cursor-not-allowed"
        >
          {isSubmitting ? 'Signing in...' : 'Sign in'}
        </button>
      </form>
    </div>
  )
}

export default Login
