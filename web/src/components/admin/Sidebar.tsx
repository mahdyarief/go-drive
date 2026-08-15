import { useAuthStore } from '@/store/auth'
import { SidebarNav } from './SidebarNav'
import { Button } from '@/components/ui/button'
import { PanelLeftClose, PanelLeft, LogOut, X } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useSidebarCollapsed } from '@/lib/useSidebarCollapsed'

interface SidebarProps {
  mobileOpen: boolean
  onClose: () => void
}

export function Sidebar({ mobileOpen, onClose }: SidebarProps) {
  const [isCollapsed, toggleCollapsed] = useSidebarCollapsed()
  const signOut = useAuthStore((s) => s.signOut)
  const user = useAuthStore((s) => s.user)

  return (
    <aside
      className={cn(
        'bg-background border-r border-border fixed inset-y-0 left-0 z-40 flex h-screen w-64 flex-col transition-transform duration-300',
        'md:static md:z-auto md:translate-x-0 md:transition-[width]',
        isCollapsed ? 'md:w-16' : 'md:w-56',
        mobileOpen ? 'translate-x-0' : '-translate-x-full',
      )}
    >
      <div className="flex h-14 items-center border-b border-border px-3">
        {(!isCollapsed || mobileOpen) ? (
          <span className="text-sm font-semibold">Admin</span>
        ) : (
          <span className="text-sm font-semibold mx-auto">A</span>
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
          onClick={onClose}
          className="ml-auto h-8 w-8 md:hidden"
          aria-label="Close menu"
        >
          <X className="h-4 w-4" />
        </Button>
      </div>

      <div className="flex-1 overflow-y-auto py-4">
        <SidebarNav isCollapsed={isCollapsed} onClose={onClose} />
      </div>

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
  )
}
