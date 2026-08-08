import { Navigate } from 'react-router'
import { useAuthStore } from '@/store/auth'

export function AdminGuard({ children }: { children: React.ReactNode }) {
  const isAdmin = useAuthStore((s) => s.isAdmin)

  if (!isAdmin) {
    return <Navigate to="/" replace />
  }

  return children
}