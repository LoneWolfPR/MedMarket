import { useState } from 'react'

type TokenResponse = { token: string }
type ApiError = { message: string }

function Login() {
  const [isPending, setIsPending] = useState(false)
  const [formError, setFormError] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [token, setToken] = useState('')
  const handleSubmit = async (e: React.SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    setFormError('')
    setIsPending(true)
    try {
      const resp = await fetch('/api/auth/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ email, password }),
      })

      if (resp.ok) {
        const data: TokenResponse = await resp.json()
        setToken(data.token)
        setEmail('')
        setPassword('')
      } else {
        const err: ApiError = await resp.json()
        setFormError(err.message)
        setPassword('')
      }
    } catch (e: unknown) {
      if (e instanceof Error) {
        setFormError('Somethign went wrong')
      } else {
        setFormError('Something went wrong')
        console.error('An unexpected error occurred: ', e)
      }
    } finally {
      setIsPending(false)
    }
  }
  const inputClass =
    'bg-white border border-slate-300 rounded-md px-3 py-2 text-sm focus:ring-teal-600 focus:ring-2 focus:border-teal-600 focus:outline-hidden'
  return (
    <div className="bg-white border border-slate-200 rounded-lg shadow-sm p-6 max-w-sm mx-auto">
      <h1 className="text-2xl font-semibold tracking-tight text-slate-900">Login</h1>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        {formError && <p className="text-sm text-red-600">{formError}</p>}
        {token && <p className="text-sm text-green-600">{token}</p>}
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
          className="w-full sm:w-auto bg-teal-600 text-white text-sm font-medium px-4 py-2 rounded-lg hover:bg-teal-700 focus:ring-teal-600 focus:ring-2 focus:ring-offset-2 focus:outline-hidden disabled:bg-slate-300 disabled:text-slate-500 disabled:cursor-not-allowed"
        >
          {isPending ? 'Signing in...' : 'Sign in'}
        </button>
      </form>
    </div>
  )
}

export default Login
