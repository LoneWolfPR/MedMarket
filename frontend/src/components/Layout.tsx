import { Link, NavLink, Outlet } from 'react-router'
import type { NavLinkRenderProps } from 'react-router'

const navLinkClass = ({ isActive }: NavLinkRenderProps) => {
  return isActive ? 'text-teal-600' : 'text-slate-600 hover:text-slate-900'
}

function Layout() {
  return (
    <div className="min-h-screen bg-slate-50">
      <header className="border-b border-slate-200 bg-white">
        <div className="flex items-center justify-between h-16 mx-auto max-w-5xl px-4 sm:px-6">
          <Link to="/" className="text-lg font-semibold tracking-tight text-slate-900">
            MedMarket
          </Link>
          <nav className="text-sm font-medium flex gap-6">
            <NavLink to="/" className={navLinkClass}>
              Home
            </NavLink>
            <NavLink to="/login" className={navLinkClass}>
              Login
            </NavLink>
          </nav>
        </div>
      </header>
      <main className="mx-auto max-w-5xl px-4 sm:px-6 py-8">
        <Outlet />
      </main>
    </div>
  )
}

export default Layout
