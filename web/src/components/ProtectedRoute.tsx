import { Navigate, useLocation } from 'react-router'
import { useAuthStore } from '@/store/auth'

export function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const user = useAuthStore((s) => s.user)
  const isAdmin = useAuthStore((s) => s.isAdmin)
  const { pathname } = useLocation()

  if (!user) {
    return <Navigate to="/login" replace />
  }

  // Admin users auto-redirect to /admin (but not from /app routes)
  if (isAdmin && !pathname.startsWith('/admin') && !pathname.startsWith('/app')) {
    return <Navigate to="/admin" replace />
  }

  return children
}
