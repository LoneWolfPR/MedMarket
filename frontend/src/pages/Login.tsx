import { useState } from 'react'
import useAuth from '../auth/useAuth'
import { useLocation, useNavigate } from 'react-router'
import { inputClass } from './sharedClasses'

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
function Login() {
  const navigate = useNavigate()
  const { state: locState } = useLocation()
  const { login } = useAuth()
  const [isPending, setIsPending] = useState(false)
  const [formError, setFormError] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const handleSubmit = async (e: React.SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    setFormError('')
    setIsPending(true)
    try {
      await login(email, password)
      const dest = isValidRedirect(locState) ? locState.from : '/'
      navigate(dest, { replace: true })
    } catch (e: unknown) {
      if (e instanceof Error) {
        setFormError(e.message)
      } else {
        setFormError('Something went wrong')
      }
      console.error('An unexpected error occurred: ', e)
    } finally {
      setIsPending(false)
    }
  }

  return (
    <div className="bg-white border border-slate-200 rounded-lg shadow-sm p-6 max-w-sm mx-auto">
      <h1 className="text-2xl font-semibold tracking-tight text-slate-900">Login</h1>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        {formError && <p className="text-sm text-red-600">{formError}</p>}
        <div className="flex flex-col gap-1.5">
          <label htmlFor="email" className="text-sm font-medium text-slate-700">
            Email
          </label>
          <input
            id="email"
            type="email"
            value={email}
            autoComplete="email"
            className={inputClass}
            onChange={(e) => setEmail(e.target.value)}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <label htmlFor="password" className="text-sm font-medium text-slate-700">
            Password
          </label>
          <input
            id="password"
            type="password"
            value={password}
            autoComplete="current-password"
            className={inputClass}
            onChange={(e) => setPassword(e.target.value)}
          />
        </div>
        <button
          type="submit"
          disabled={isPending}
          className="w-full sm:w-auto bg-teal-600 text-white text-sm font-medium px-4 py-2 rounded-lg cursor-pointer hover:bg-teal-700 focus:ring-teal-600 focus:ring-2 focus:ring-offset-2 focus:outline-hidden disabled:bg-slate-300 disabled:text-slate-500 disabled:cursor-not-allowed"
        >
          {isPending ? 'Signing in...' : 'Sign in'}
        </button>
      </form>
    </div>
  )
}

export default Login
