import { useState } from 'react'
import { Sidebar } from './Sidebar'
import { Button } from '@/components/ui/button'
import { Menu } from 'lucide-react'

export function AdminLayout({ children }: { children: React.ReactNode }) {
  const [mobileOpen, setMobileOpen] = useState(false)

  return (
    <div className="flex h-screen overflow-hidden">
      {mobileOpen && (
        <div className="fixed inset-0 z-30 bg-black/50 md:hidden" onClick={() => setMobileOpen(false)} />
      )}
      <Sidebar mobileOpen={mobileOpen} onClose={() => setMobileOpen(false)} />
      <main className="flex-1 overflow-y-auto">
        <div className="flex items-center border-b border-border px-3 py-2 md:hidden">
          <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => setMobileOpen(true)} aria-label="Open menu">
            <Menu className="h-5 w-5" />
          </Button>
        </div>
        <div className="mx-auto max-w-6xl p-4 md:p-6">
          {children}
        </div>
      </main>
    </div>
  )
}
