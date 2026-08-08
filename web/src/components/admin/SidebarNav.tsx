import { NavLink } from 'react-router'
import { Settings, Building2, Users } from 'lucide-react'
import { useAuthStore } from '@/store/auth'
import { cn } from '@/lib/utils'

const adminNavItems = [
  { href: '/admin/organizations', label: 'Organizations', icon: Building2 },
  { href: '/admin/users', label: 'Users', icon: Users },
  { href: '/admin/settings', label: 'Settings', icon: Settings },
]

type NavItem = (typeof adminNavItems)[number]

function NavLinkItem({ item, isCollapsed }: { item: NavItem; isCollapsed: boolean }) {
  return (
    <NavLink
      key={item.href}
      to={item.href}
      end
      className={({ isActive }) =>
        cn(
          'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors',
          isActive
            ? 'bg-accent text-accent-foreground'
            : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground',
          isCollapsed && 'md:justify-center md:px-2',
        )
      }
    >
      <item.icon className="h-4 w-4 shrink-0" />
      <span className={cn(isCollapsed && 'md:hidden')}>{item.label}</span>
    </NavLink>
  )
}

export function SidebarNav({ isCollapsed }: { isCollapsed: boolean }) {
  const isAdmin = useAuthStore((s) => s.isAdmin)

  if (!isAdmin) return null

  return (
    <nav className="flex flex-col gap-1 px-2">
      <p
        className={cn(
          'px-3 pb-1 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/70',
          isCollapsed && 'md:hidden',
        )}
      >
        Administration
      </p>
      {adminNavItems.map((item) => (
        <NavLinkItem key={item.href} item={item} isCollapsed={isCollapsed} />
      ))}
    </nav>
  )
}
