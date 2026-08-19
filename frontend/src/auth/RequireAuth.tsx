import { Navigate, Outlet, useLocation } from 'react-router'
import useAuth from './useAuth'

export default function RequireAuth() {
  const { isAuthenticated } = useAuth()
  const location = useLocation()
  if (isAuthenticated) {
    return <Outlet />
  }
  const path = location.pathname + location.search
  return <Navigate to="/login" state={{ from: path }} replace />
}
