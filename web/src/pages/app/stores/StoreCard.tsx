import { useTranslation } from 'react-i18next'
import type { Store } from '@/lib/types'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'
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
  onIngest,
  ingestPending,
  onDelete,
}: StoreCardProps) {
  const { t } = useTranslation()
  const Icon = PROVIDER_ICONS[store.provider as keyof typeof PROVIDER_ICONS] ?? Database

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
            <p className="truncate text-xs text-muted-foreground">
              {providerLabel(t, store.provider)}
              {store.quota_limit > 0 && ` · ${formatBytes(store.quota_used)} / ${formatBytes(store.quota_limit)}`}
              {store.write_mode !== 'none' && ` · ${store.write_mode}`}
            </p>
          </div>
        </div>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="h-8 w-8 shrink-0" aria-label={t('stores.moreActions')}>
              <MoreHorizontal className="h-4 w-4" />
            </Button>
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
      {store.quota_limit > 0 && store.status === 'active' && (
        <Progress value={Math.min(100, (store.quota_used / store.quota_limit) * 100)} className="mt-2 h-1" />
      )}
    </div>
  )
}
