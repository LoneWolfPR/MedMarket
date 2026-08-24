import { Link } from 'react-router'

function NotFound() {
  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-2xl font-semibold tracking-tight text-slate-900">Page not found</h1>
      <p className="text-slate-600">That page doesn&rsquo;t exist, or it may have moved.</p>
      <Link
        to="/"
        className="text-sm font-medium text-teal-600 underline hover:text-teal-700 focus:ring-2 focus:ring-teal-600 focus:outline-hidden"
      >
        Back to home
      </Link>
    </div>
  )
}

export default NotFound
