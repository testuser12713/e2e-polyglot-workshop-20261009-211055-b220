import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { useAuth } from './AuthContext'

/**
 * Route guard for the workshop area. Renders its child route only when a token
 * is present, otherwise redirects to the workshop login and remembers where the
 * user was headed.
 */
export default function RequireAuth() {
  const { token } = useAuth()
  const location = useLocation()

  if (!token) {
    return <Navigate to="/werkstatt/login" replace state={{ from: location.pathname }} />
  }

  return <Outlet />
}
