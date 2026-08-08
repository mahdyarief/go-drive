import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { applyTheme, usePreferencesStore, type Theme } from '@/store/preferences'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Monitor, Moon, Sun } from 'lucide-react'

const THEME_OPTIONS: { value: Theme; icon: typeof Sun; labelKey: string }[] = [
  { value: 'light', icon: Sun, labelKey: 'settings.appearance.light' },
  { value: 'dark', icon: Moon, labelKey: 'settings.appearance.dark' },
  { value: 'system', icon: Monitor, labelKey: 'settings.appearance.system' },
]

export default function SettingsAppearancePage() {
  const { t } = useTranslation()
  const theme = usePreferencesStore((s) => s.theme)
  const setTheme = usePreferencesStore((s) => s.setTheme)

  // Applying the stored theme to the DOM is an external-system sync.
  useEffect(() => {
    applyTheme(theme)
  }, [theme])

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t('settings.appearance.title')}</h1>
        <p className="text-sm text-muted-foreground">{t('settings.appearance.description')}</p>
      </div>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">{t('settings.appearance.theme')}</CardTitle>
          <CardDescription>{t('settings.appearance.themeHint')}</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap items-center gap-3">
            <Select value={theme} onValueChange={(v) => setTheme(v as Theme)}>
              <SelectTrigger className="w-48" aria-label={t('settings.appearance.theme')}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {THEME_OPTIONS.map((option) => {
                  const Icon = option.icon
                  return (
                    <SelectItem key={option.value} value={option.value}>
                      <span className="flex items-center gap-2">
                        <Icon className="h-4 w-4" />
                        {t(option.labelKey)}
                      </span>
                    </SelectItem>
                  )
                })}
              </SelectContent>
            </Select>
            <Label className="text-sm text-muted-foreground">{t(`settings.appearance.${theme}`)}</Label>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
