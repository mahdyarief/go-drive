import { Navigate } from 'react-router'
import { useOrgStore } from '@/store/org'
import { useAuthStore } from '@/store/auth'

export function OrgGuard({ children }: { children: React.ReactNode }) {
  const currentOrg = useOrgStore((s) => s.currentOrg)
  const organizations = useOrgStore((s) => s.organizations)
  const isAdmin = useAuthStore((s) => s.isAdmin)

  // Admin users can access admin pages without an org
  if (isAdmin) {
    return children
  }

  if (organizations.length === 0 || !currentOrg) {
    return <Navigate to="/orgs" replace />
  }

  return children
}