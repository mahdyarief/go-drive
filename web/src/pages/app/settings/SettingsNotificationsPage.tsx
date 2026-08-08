import { useTranslation } from 'react-i18next'
import { usePreferencesStore } from '@/store/preferences'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { BellRing, Download, HardDrive, Share2, UserPlus } from 'lucide-react'

const NOTIFICATION_ITEMS = [
  { key: 'shareActivity', icon: Share2, label: 'settings.notifications.shareActivity', hint: 'settings.notifications.shareActivityHint' },
  { key: 'downloads', icon: Download, label: 'settings.notifications.downloads', hint: 'settings.notifications.downloadsHint' },
  { key: 'newMembers', icon: UserPlus, label: 'settings.notifications.newMembers', hint: 'settings.notifications.newMembersHint' },
  { key: 'storage', icon: HardDrive, label: 'settings.notifications.storage', hint: 'settings.notifications.storageHint' },
] as const

export default function SettingsNotificationsPage() {
  const { t } = useTranslation()
  const shareActivity = usePreferencesStore((s) => s.shareActivity)
  const downloads = usePreferencesStore((s) => s.downloads)
  const newMembers = usePreferencesStore((s) => s.newMembers)
  const storage = usePreferencesStore((s) => s.storage)
  const setNotificationPref = usePreferencesStore((s) => s.setNotificationPref)

  const values: Record<(typeof NOTIFICATION_ITEMS)[number]['key'], boolean> = {
    shareActivity,
    downloads,
    newMembers,
    storage,
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t('settings.notifications.title')}</h1>
        <p className="text-sm text-muted-foreground">{t('settings.notifications.description')}</p>
      </div>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-base">
            <BellRing className="h-4 w-4" />
            {t('settings.notifications.title')}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {NOTIFICATION_ITEMS.map((item) => {
            const Icon = item.icon
            return (
              <div key={item.key} className="flex items-center justify-between gap-4">
                <div className="flex items-start gap-3">
                  <Icon className="h-4 w-4 mt-0.5 text-muted-foreground" />
                  <div>
                    <Label htmlFor={`notif-${item.key}`}>{t(item.label)}</Label>
                    <p className="text-xs text-muted-foreground">{t(item.hint)}</p>
                  </div>
                </div>
                <Switch
                  id={`notif-${item.key}`}
                  checked={values[item.key]}
                  onCheckedChange={(v) => setNotificationPref(item.key, v)}
                />
              </div>
            )
          })}
        </CardContent>
      </Card>
    </div>
  )
}
