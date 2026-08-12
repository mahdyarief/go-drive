import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/store/auth'
import { OrgSwitcher } from '@/components/OrgSwitcher'
import { NavLink } from 'react-router'
import { Button } from '@/components/ui/button'
import { PanelLeftClose, PanelLeft, LogOut, Home, LayoutDashboard, X, Menu, Folder, Database, Link2, Users, Palette, Bell, BookOpen, History } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useSidebarCollapsed } from '@/lib/useSidebarCollapsed'
import { AppHeader } from '@/components/app/AppHeader'
import { StorageUsageSidebar } from '@/components/app/StorageUsageSidebar'
import { UploadPanel } from '@/components/app/UploadPanel'
import { RecentFolders } from '@/components/app/RecentFolders'

export function AppLayout({ children }: { children: React.ReactNode }) {
  const { t } = useTranslation()
  const [isCollapsed, toggleCollapsed] = useSidebarCollapsed()
  const [mobileOpen, setMobileOpen] = useState(false)
  const signOut = useAuthStore((s) => s.signOut)
  const user = useAuthStore((s) => s.user)

  const linkClass = ({ isActive }: { isActive: boolean }) =>
    cn(
      'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors',
      isActive
        ? 'bg-accent text-accent-foreground'
        : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground',
      isCollapsed && 'md:justify-center md:px-2',
    )

  return (
    <div className="flex h-screen overflow-hidden">
      {mobileOpen && (
        <div className="fixed inset-0 z-30 bg-black/50 md:hidden" onClick={() => setMobileOpen(false)} />
      )}
      <aside
        className={cn(
          'bg-background border-r border-border fixed inset-y-0 left-0 z-40 flex h-screen w-64 flex-col transition-transform duration-300',
          'md:static md:z-auto md:translate-x-0 md:transition-[width]',
          isCollapsed ? 'md:w-16' : 'md:w-56',
          mobileOpen ? 'translate-x-0' : '-translate-x-full',
        )}
      >
        <div className="flex h-14 items-center border-b border-border px-3">
          {(!isCollapsed || mobileOpen) && (
            <div className="flex-1 min-w-0">
              <OrgSwitcher />
            </div>
          )}
          <Button
            variant="ghost"
            size="icon"
            onClick={toggleCollapsed}
            className="ml-auto h-8 w-8 hidden md:inline-flex"
          >
            {isCollapsed ? <PanelLeft className="h-4 w-4" /> : <PanelLeftClose className="h-4 w-4" />}
          </Button>
          <Button
            variant="ghost"
            size="icon"
            onClick={() => setMobileOpen(false)}
            className="ml-auto h-8 w-8 md:hidden"
            aria-label="Close menu"
          >
            <X className="h-4 w-4" />
          </Button>
        </div>

        <nav className="flex-1 overflow-y-auto py-4 px-2">
          <NavLink to="/" end className={linkClass}>
            <Home className="h-4 w-4 shrink-0" />
            <span className={cn(isCollapsed && 'md:hidden')}>Home</span>
          </NavLink>
          <NavLink to="/app/status" end className={linkClass}>
            <LayoutDashboard className="h-4 w-4 shrink-0" />
            <span className={cn(isCollapsed && 'md:hidden')}>Dashboard</span>
          </NavLink>
          <NavLink to="/app/files" end className={linkClass}>
            <Folder className="h-4 w-4 shrink-0" />
            <span className={cn(isCollapsed && 'md:hidden')}>Files</span>
          </NavLink>
          <RecentFolders collapsed={isCollapsed} />
          <NavLink to="/app/links" end className={linkClass}>
            <Link2 className="h-4 w-4 shrink-0" />
            <span className={cn(isCollapsed && 'md:hidden')}>Links</span>
          </NavLink>
          <NavLink to="/app/activity" end className={linkClass}>
            <History className="h-4 w-4 shrink-0" />
            <span className={cn(isCollapsed && 'md:hidden')}>{t('activity.nav')}</span>
          </NavLink>
          <div className={cn('mt-4 px-3 text-xs font-semibold uppercase tracking-wide text-muted-foreground', isCollapsed && 'md:hidden')}>
            Settings
          </div>
          <NavLink to="/app/settings/stores" end className={linkClass}>
            <Database className="h-4 w-4 shrink-0" />
            <span className={cn(isCollapsed && 'md:hidden')}>Stores</span>
          </NavLink>
          <NavLink to="/app/settings/members" end className={linkClass}>
            <Users className="h-4 w-4 shrink-0" />
            <span className={cn(isCollapsed && 'md:hidden')}>Members</span>
          </NavLink>
          <NavLink to="/app/settings/appearance" end className={linkClass}>
            <Palette className="h-4 w-4 shrink-0" />
            <span className={cn(isCollapsed && 'md:hidden')}>Appearance</span>
          </NavLink>
          <NavLink to="/app/settings/notifications" end className={linkClass}>
            <Bell className="h-4 w-4 shrink-0" />
            <span className={cn(isCollapsed && 'md:hidden')}>Notifications</span>
          </NavLink>
          <NavLink to="/app/api-docs" end className={linkClass}>
            <BookOpen className="h-4 w-4 shrink-0" />
            <span className={cn(isCollapsed && 'md:hidden')}>{t('apiDocs.nav')}</span>
          </NavLink>
        </nav>

        <StorageUsageSidebar collapsed={isCollapsed} />

        <div className="border-t border-border p-3">
          <div className={cn('flex items-center gap-2', isCollapsed && 'md:justify-center')}>
            <div className="h-7 w-7 rounded-full bg-muted flex items-center justify-center text-xs font-medium">
              {user?.name?.charAt(0) || 'U'}
            </div>
            {(!isCollapsed || mobileOpen) && (
              <div className="flex-1 min-w-0">
                <p className="text-xs font-medium truncate">{user?.name}</p>
                <p className="text-[10px] text-muted-foreground truncate">{user?.email}</p>
              </div>
            )}
            {(!isCollapsed || mobileOpen) && (
              <Button variant="ghost" size="icon" className="h-7 w-7" onClick={signOut}>
                <LogOut className="h-3.5 w-3.5" />
              </Button>
            )}
          </div>
        </div>
      </aside>

      <main className="flex-1 overflow-y-auto">
        <div className="flex items-center border-b border-border px-3 py-2 md:hidden">
          <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => setMobileOpen(true)} aria-label="Open menu">
            <Menu className="h-5 w-5" />
          </Button>
        </div>
        <AppHeader />
        <div className="mx-auto max-w-6xl p-4 md:p-6">
          {children}
        </div>
      </main>
      <UploadPanel />
    </div>
  )
}
