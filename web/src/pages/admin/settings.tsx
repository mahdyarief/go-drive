import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { adminApi } from '@/lib/api'
import type { RegisterSettings } from '@/lib/types'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import { Settings } from 'lucide-react'

export default function AdminSettingsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const { data: register, isLoading: registerLoading, isError: registerError } = useQuery({
    queryKey: ['admin', 'settings', 'register'],
    queryFn: () => adminApi<RegisterSettings>('/api/admin/settings/register'),
  })

  const saveRegister = useMutation({
    mutationFn: (disabled: boolean) =>
      adminApi<void>('/api/admin/settings/register', {
        method: 'PUT',
        body: JSON.stringify({ register_disabled: disabled }),
      }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'settings', 'register'] }),
  })

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <div className="rounded-full bg-primary/10 p-2">
          <Settings className="h-5 w-5 text-primary" />
        </div>
        <h1 className="text-2xl font-bold">{t('settings.title')}</h1>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t('settings.register.title')}</CardTitle>
          <CardDescription>{t('settings.register.description')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {registerError && <p className="text-sm text-destructive">{t('settings.register.loadError')}</p>}

          <div className="flex items-center justify-between gap-4">
            <div className="space-y-1">
              <p className="text-sm font-medium">{t('settings.register.disableLabel')}</p>
              <p className="text-xs text-muted-foreground">{t('settings.register.disableHint')}</p>
            </div>
            <Switch
              checked={register?.register_disabled ?? false}
              onCheckedChange={(checked) => saveRegister.mutate(checked)}
              disabled={registerLoading || saveRegister.isPending}
              aria-label={t('settings.register.disableLabel')}
            />
          </div>

          {saveRegister.isError && (
            <p className="text-sm text-destructive">{t('settings.register.saveError')}</p>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
