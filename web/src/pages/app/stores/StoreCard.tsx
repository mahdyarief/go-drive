import { useTranslation } from 'react-i18next'
import type { Store } from '@/lib/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Check, Database, MoreHorizontal, Pencil, RefreshCw, Star, Trash2, Zap } from 'lucide-react'
import { PROVIDER_ICONS, formatBytes, providerLabel } from './stores'

interface StoreCardProps {
  store: Store
  primaryStoreId: string | null
  onEdit: (store: Store) => void
  onGdriveConnect: (storeId: string) => void
  gdriveAuthPending: boolean
  onSetPrimary: (storeId: string) => void
  setPrimaryPending: boolean
  onTest: (storeId: string) => void
  testPending: boolean
  onRefreshQuota: (storeId: string) => void
  refreshQuotaPending: boolean
  onIngest: (storeId: string) => void
  ingestPending: boolean
  onDelete: (store: Store) => void
}

// StoreCard renders a single store as a compact row: provider icon, name,
// status badges, a muted summary line, and an actions dropdown on the right.
// The thin progress bar shows quota usage when a limit is set.
export function StoreCard({
  store,
  primaryStoreId,
  onEdit,
  onGdriveConnect,
  gdriveAuthPending,
  onSetPrimary,
  setPrimaryPending,
  onTest,
  testPending,
  onRefreshQuota,
  refreshQuotaPending,
  onIngest,
  ingestPending,
  onDelete,
}: StoreCardProps) {
  const { t } = useTranslation()
  const Icon = PROVIDER_ICONS[store.provider as keyof typeof PROVIDER_ICONS] ?? Database

  // quotaRows renders one bar per available quota: the provider's own capacity
  // (e.g. Google Drive's 15 GB) and the app-configured storage limit. Each bar
  // clamps at 100% and turns destructive when used > limit.
  const quotaRows: { label: string; limit: number }[] = []
  if (store.provider_quota_limit > 0) {
    quotaRows.push({ label: t('stores.driveQuota'), limit: store.provider_quota_limit })
  }
  if (store.quota_limit > 0) {
    quotaRows.push({ label: t('stores.storageLimit'), limit: store.quota_limit })
  }

  return (
    <div className="rounded-lg border px-3 py-2">
      <div className="flex items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <span className="truncate text-sm font-medium">{store.name}</span>
              {store.provider === 'gdrive' && store.status === 'active' && (
                <Badge variant="outline" className="gap-1 border-emerald-600/30 bg-emerald-600/10 text-emerald-700">
                  <Check className="h-3 w-3" />
                  {t('stores.connected')}
                </Badge>
              )}
              {store.provider === 'gdrive' && store.status === 'pending' && (
                <Badge variant="outline" className="gap-1 border-amber-500/30 bg-amber-500/10 text-amber-700">
                  {t('stores.pending')}
                </Badge>
              )}
              {store.provider === 'gdrive' && store.status === 'error' && (
                <Badge variant="destructive" className="gap-1">
                  {t('stores.error')}
                </Badge>
              )}
              {store.id === primaryStoreId && (
                <Badge className="gap-1">
                  <Star className="h-3 w-3" />
                  {t('stores.primary')}
                </Badge>
              )}
            </div>
          </div>
        </div>
        <DropdownMenu>
          <DropdownMenuTrigger
            render={<Button variant="ghost" size="icon" className="h-8 w-8 shrink-0" aria-label={t('stores.moreActions')} />}
          >
            <MoreHorizontal className="h-4 w-4" />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="min-w-44">
            <DropdownMenuItem onClick={() => onEdit(store)}>
              <Pencil className="h-4 w-4 mr-2" />
              {t('stores.editStore')}
            </DropdownMenuItem>
            {store.provider === 'gdrive' && store.status !== 'active' && (
              <DropdownMenuItem disabled={gdriveAuthPending} onClick={() => onGdriveConnect(store.id)}>
                <RefreshCw className="h-4 w-4 mr-2" />
                {t('stores.connectGoogle')}
              </DropdownMenuItem>
            )}
            {store.id !== primaryStoreId && (
              <DropdownMenuItem disabled={setPrimaryPending} onClick={() => onSetPrimary(store.id)}>
                <Star className="h-4 w-4 mr-2" />
                {t('stores.setPrimary')}
              </DropdownMenuItem>
            )}
            <DropdownMenuItem disabled={testPending} onClick={() => onTest(store.id)}>
              <Check className="h-4 w-4 mr-2" />
              {t('stores.testConnection')}
            </DropdownMenuItem>
            <DropdownMenuItem disabled={refreshQuotaPending} onClick={() => onRefreshQuota(store.id)}>
              <RefreshCw className="h-4 w-4 mr-2" />
              {t('stores.refreshQuota')}
            </DropdownMenuItem>
            <DropdownMenuItem disabled={ingestPending} onClick={() => onIngest(store.id)}>
              <Zap className="h-4 w-4 mr-2" />
              {t('stores.triggerIngest')}
            </DropdownMenuItem>
            <DropdownMenuItem className="text-destructive" onClick={() => onDelete(store)}>
              <Trash2 className="h-4 w-4 mr-2" />
              {t('stores.deleteStore')}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      {quotaRows.length > 0 && (
        <div className="mt-3 space-y-2">
          {quotaRows.map((row) => {
            const percent = Math.min(100, (store.quota_used / row.limit) * 100)
            const over = store.quota_used > row.limit
            return (
              <div key={row.label} className="space-y-1.5">
                <div className="h-2.5 w-full overflow-hidden rounded-full bg-muted">
                  <div
                    className={`h-full ${over ? 'bg-destructive' : 'bg-primary'}`}
                    style={{ width: `${percent}%` }}
                  />
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-xs text-muted-foreground">{row.label}</span>
                  <span className={`text-xs font-medium tabular-nums ${over ? 'text-destructive' : 'text-muted-foreground'}`}>
                    {formatBytes(store.quota_used)} / {formatBytes(row.limit)}
                  </span>
                </div>
              </div>
            )
          })}
        </div>
      )}
      <div className="mt-2 flex items-center justify-between gap-3 text-xs text-muted-foreground">
        <p className="min-w-0 truncate">
          {providerLabel(t, store.provider)}
          {store.write_mode !== 'none' && ` · ${store.write_mode}`}
        </p>
        {store.provider === 'gdrive' && (
          <p className="shrink-0 text-muted-foreground/70">
            {store.provider_quota_measured_at
              ? t('stores.quotaMeasuredAt', { time: new Date(store.provider_quota_measured_at).toLocaleString() })
              : t('stores.quotaNotMeasured')}
          </p>
        )}
      </div>
    </div>
  )
}
