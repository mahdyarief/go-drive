import { useState } from 'react'
import { useNavigate } from 'react-router'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { usePreferencesStore, type Theme } from '@/store/preferences'
import { Moon, Search, Sun } from 'lucide-react'

const NEXT_THEME: Record<Theme, Theme> = { light: 'dark', dark: 'light', system: 'light' }

// AppHeader is the global top bar rendered inside AppLayout on every app page.
// Search navigates to the files page with a ?q= param; the theme button is a
// quick light/dark toggle backed by the global preferences store.
export function AppHeader() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const theme = usePreferencesStore((s) => s.theme)
  const setTheme = usePreferencesStore((s) => s.setTheme)
  const [query, setQuery] = useState('')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    const q = query.trim()
    navigate(q ? `/app/files?q=${encodeURIComponent(q)}` : '/app/files')
  }

  const toggleTheme = () => setTheme(NEXT_THEME[theme])

  return (
    <div className="flex items-center gap-2 border-b border-border px-3 py-2 md:px-4">
      <form onSubmit={handleSubmit} className="relative max-w-md flex-1">
        <Search className="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t('app.searchPlaceholder')}
          className="pl-8"
          aria-label={t('app.searchPlaceholder')}
        />
      </form>
      <Button
        variant="ghost"
        size="icon"
        className="ml-auto h-8 w-8"
        onClick={toggleTheme}
        aria-label={t('app.toggleTheme')}
      >
        {theme === 'dark' ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
      </Button>
    </div>
  )
}
