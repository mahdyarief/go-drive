import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { adminApi } from '@/lib/api'
import type { GDriveSettings, GDriveStorageQuota, RegisterSettings } from '@/lib/types'
import { Button, buttonVariants } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { ExternalLink, HelpCircle, Settings } from 'lucide-react'

function formatBytes(bytes: number) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)))
  return `${(bytes / 1024 ** i).toFixed(1)} ${units[i]}`
}

export default function AdminSettingsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const [clientId, setClientId] = useState('')
  const [clientSecret, setClientSecret] = useState('')
  const [folderId, setFolderId] = useState('')
  const [showHelp, setShowHelp] = useState(false)

  const { data: gdrive, isLoading, isError } = useQuery({
    queryKey: ['admin', 'settings', 'gdrive'],
    queryFn: () => adminApi<GDriveSettings>('/api/admin/settings/gdrive'),
  })

  const save = useMutation({
    mutationFn: () =>
      adminApi<void>('/api/admin/settings/gdrive', {
        method: 'PUT',
        body: JSON.stringify({ client_id: clientId, client_secret: clientSecret, folder_id: folderId }),
      }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'settings', 'gdrive'] }),
  })

  const connect = useMutation({
    mutationFn: () => adminApi<{ auth_url: string }>('/api/admin/settings/gdrive/auth-url', { method: 'POST' }),
    onSuccess: (data) => {
      if (data.auth_url) window.open(data.auth_url, '_blank', 'noopener,noreferrer')
    },
  })

  const disconnect = useMutation({
    mutationFn: () => adminApi<void>('/api/admin/settings/gdrive/disconnect', { method: 'POST' }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'settings', 'gdrive'] }),
  })

  const { data: storage, isError: storageError } = useQuery({
    queryKey: ['admin', 'settings', 'gdrive', 'storage'],
    queryFn: () => adminApi<GDriveStorageQuota>('/api/admin/settings/gdrive/storage'),
    enabled: !!gdrive?.connected,
  })

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
          <div className="flex items-center justify-between">
            <CardTitle>{t('settings.gdrive.title')}</CardTitle>
            <div className="flex items-center gap-2">
              <Button variant="ghost" size="sm" className="h-7" onClick={() => setShowHelp(true)}>
                <HelpCircle className="h-3.5 w-3.5 mr-1" />
                {t('settings.gdrive.helpTrigger')}
              </Button>
              {isLoading ? (
                <Badge variant="secondary">{t('app.loading')}</Badge>
              ) : (
                <Badge variant="secondary" className={gdrive?.connected ? 'bg-emerald-500/10 text-emerald-600' : ''}>
                  {gdrive?.connected ? t('settings.gdrive.connected') : t('settings.gdrive.notConnected')}
                </Badge>
              )}
            </div>
          </div>
          <CardDescription>{t('settings.gdrive.description')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {isError && <p className="text-sm text-destructive">{t('settings.gdrive.loadError')}</p>}

          <div className="space-y-2">
            <Label htmlFor="client-id">{t('settings.gdrive.clientId')}</Label>
            <Input
              id="client-id"
              type="text"
              placeholder="1234567890-abc.apps.googleusercontent.com"
              value={clientId}
              onChange={(e) => setClientId(e.target.value)}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="client-secret">{t('settings.gdrive.clientSecret')}</Label>
            <Input
              id="client-secret"
              type="password"
              placeholder="••••••••"
              value={clientSecret}
              onChange={(e) => setClientSecret(e.target.value)}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="folder-id">{t('settings.gdrive.folderId')}</Label>
            <Input
              id="folder-id"
              type="text"
              placeholder="1_k5iWEuL8LZkB054G5GLM7UMvckyVhT0"
              value={folderId}
              onChange={(e) => setFolderId(e.target.value)}
            />
          </div>

          <div className="flex flex-wrap gap-2">
            <Button onClick={() => save.mutate()} disabled={save.isPending || !clientId || !folderId}>
              {t('settings.gdrive.save')}
            </Button>
            <Button variant="outline" onClick={() => connect.mutate()} disabled={connect.isPending || !gdrive?.configured}>
              {t('settings.gdrive.connect')}
            </Button>
            <Button
              variant="ghost"
              className="text-destructive"
              onClick={() => disconnect.mutate()}
              disabled={disconnect.isPending || !gdrive?.connected}
            >
              {t('settings.gdrive.disconnect')}
            </Button>
          </div>

          {save.isError && <p className="text-sm text-destructive">{t('settings.gdrive.saveError')}</p>}
          {save.isSuccess && <p className="text-sm text-emerald-600">{t('settings.gdrive.saveSuccess')}</p>}
          {connect.isError && <p className="text-sm text-destructive">{t('settings.gdrive.connectError')}</p>}

          {gdrive?.connected &&
            (storage ? (
              <div className="space-y-1.5">
                <div className="flex items-center justify-between text-xs text-muted-foreground">
                  <span>{t('settings.gdrive.storageUsed')}</span>
                  <span>
                    {formatBytes(storage.usage)} / {formatBytes(storage.limit)}
                  </span>
                </div>
                <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
                  <div
                    className="h-full rounded-full bg-primary"
                    style={{ width: `${Math.min(100, (storage.usage / storage.limit) * 100)}%` }}
                  />
                </div>
              </div>
            ) : storageError ? (
              <p className="text-xs text-muted-foreground">{t('settings.gdrive.storageReauthHint')}</p>
            ) : null)}

          {gdrive?.redirect_uri && (
            <div className="rounded-lg bg-muted p-3 text-xs text-muted-foreground">
              <p className="font-medium">{t('settings.gdrive.redirectUriHint')}</p>
              <code className="mt-1 block break-all">{gdrive.redirect_uri}</code>
            </div>
          )}
        </CardContent>
      </Card>

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

      <Dialog open={showHelp} onOpenChange={(open) => !open && setShowHelp(false)}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{t('settings.gdrive.helpTitle')}</DialogTitle>
            <DialogDescription>{t('settings.gdrive.helpSubtitle')}</DialogDescription>
          </DialogHeader>
          <ol className="list-decimal space-y-2.5 pl-5 text-sm marker:font-medium marker:text-primary">
            <li>{t('settings.gdrive.helpStep1')}</li>
            <li>{t('settings.gdrive.helpStep2')}</li>
            <li>{t('settings.gdrive.helpStep3')}</li>
            <li>{t('settings.gdrive.helpStep4')}</li>
            <li>
              {t('settings.gdrive.helpStep5')}
              {gdrive?.redirect_uri && (
                <code className="mt-2 block break-all rounded-md bg-muted px-2 py-1 text-xs">{gdrive.redirect_uri}</code>
              )}
            </li>
            <li>{t('settings.gdrive.helpStep6')}</li>
          </ol>
          <div className="flex justify-end">
            <a
              href="https://console.cloud.google.com/"
              target="_blank"
              rel="noreferrer"
              className={buttonVariants({ variant: 'outline', size: 'sm' })}
            >
              <ExternalLink className="h-3.5 w-3.5 mr-1" />
              {t('settings.gdrive.openConsole')}
            </a>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
