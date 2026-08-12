import { BrowserRouter, Routes, Route, Navigate, Outlet } from 'react-router'
import { QueryClientProvider, useQuery } from '@tanstack/react-query'
import { queryClient } from '@/lib/query'
import { api } from '@/lib/api'
import type { MeData } from '@/lib/types'
import { useAuthStore } from '@/store/auth'
import { useOrgStore } from '@/store/org'
import { ProtectedRoute } from '@/components/ProtectedRoute'
import { OrgGuard } from '@/components/OrgGuard'
import { AdminGuard } from '@/components/AdminGuard'
import { AdminLayout } from '@/components/admin/AdminLayout'
import { AppLayout } from '@/components/app/AppLayout'
import OrganizationsPage from '@/pages/admin/organizations'
import UsersPage from '@/pages/admin/users'
import AdminSettingsPage from '@/pages/admin/settings'
import LoginPage from '@/pages/LoginPage'
import SignupPage from '@/pages/SignupPage'
import OrgsPage from '@/pages/OrgsPage'
import OrgSettingsPage from '@/pages/OrgSettingsPage'
import DashboardPage from '@/pages/app/DashboardPage'
import FilesPage from '@/pages/app/FilesPage'
import FilePreviewPage from '@/pages/app/FilePreviewPage'
import StoresPage from '@/pages/app/StoresPage'
import LinksPage from '@/pages/app/LinksPage'
import ActivityLogPage from '@/pages/app/ActivityLogPage'
import ApiDocsPage from '@/pages/app/ApiDocsPage'
import SettingsMembersPage from '@/pages/app/settings/SettingsMembersPage'
import SettingsAppearancePage from '@/pages/app/settings/SettingsAppearancePage'
import SettingsNotificationsPage from '@/pages/app/settings/SettingsNotificationsPage'
import SharedLinkPage from '@/pages/public/SharedLinkPage'
import UploadLinkPage from '@/pages/public/UploadLinkPage'
import TrackedLinkPage from '@/pages/public/TrackedLinkPage'
import { Toaster } from '@/components/ui/sonner'

function AuthLoader({ children }: { children: React.ReactNode }) {
  const setUser = useAuthStore((s) => s.setUser)
  const setToken = useAuthStore((s) => s.setToken)
  const setOrganizations = useOrgStore((s) => s.setOrganizations)

  const { isPending } = useQuery({
    queryKey: ['auth', 'me'],
    queryFn: async () => {
      try {
        const data = await api<MeData>('/api/me')
        setUser(data.user)
        setOrganizations(data.organizations ?? [])
        return data
      } catch {
        setUser(null)
        setOrganizations([])
        setToken(null)
        return null
      }
    },
  })

  if (isPending) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background">
        <p className="text-muted-foreground">Loading...</p>
      </div>
    )
  }

  return children
}

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <Toaster />
      <BrowserRouter>
        <AuthLoader>
          <Routes>
            <Route path="/login" element={<LoginPage />} />
            <Route path="/signup" element={<SignupPage />} />
            <Route path="/shared/:token" element={<SharedLinkPage />} />
            <Route path="/upload/:token" element={<UploadLinkPage />} />
            <Route path="/tracked/:token" element={<TrackedLinkPage />} />
            <Route
              path="/orgs"
              element={
                <ProtectedRoute>
                  <OrgsPage />
                </ProtectedRoute>
              }
            />
            <Route
              path="/orgs/:slug/settings"
              element={
                <ProtectedRoute>
                  <OrgSettingsPage />
                </ProtectedRoute>
              }
            />
            <Route
              path="/"
              element={
                <ProtectedRoute>
                  <OrgGuard>
                    <Navigate to="/app/status" replace />
                  </OrgGuard>
                </ProtectedRoute>
              }
            />
            <Route
              path="/app"
              element={
                <ProtectedRoute>
                  <OrgGuard>
                    <AppLayout>
                      <Outlet />
                    </AppLayout>
                  </OrgGuard>
                </ProtectedRoute>
              }
            >
              <Route index element={<Navigate to="status" replace />} />
              <Route path="status" element={<DashboardPage />} />
              <Route path="files" element={<FilesPage />} />
              <Route path="files/preview/:fileId" element={<FilePreviewPage />} />
              <Route path="settings/stores" element={<StoresPage />} />
              <Route path="settings/members" element={<SettingsMembersPage />} />
              <Route path="settings/appearance" element={<SettingsAppearancePage />} />
              <Route path="settings/notifications" element={<SettingsNotificationsPage />} />
              <Route path="links" element={<LinksPage />} />
              <Route path="activity" element={<ActivityLogPage />} />
              <Route path="api-docs" element={<ApiDocsPage />} />
            </Route>
            <Route path="*" element={<Navigate to="/" replace />} />
            <Route
              path="/admin"
              element={
                <ProtectedRoute>
                  <AdminGuard>
                    <AdminLayout>
                      <Outlet />
                    </AdminLayout>
                  </AdminGuard>
                </ProtectedRoute>
              }
            >
              <Route index element={<Navigate to="organizations" replace />} />
              <Route path="organizations" element={<OrganizationsPage />} />
              <Route path="users" element={<UsersPage />} />
              <Route path="settings" element={<AdminSettingsPage />} />
            </Route>
          </Routes>
        </AuthLoader>
      </BrowserRouter>
    </QueryClientProvider>
  )
}

export default App
