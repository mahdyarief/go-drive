import { useTranslation } from 'react-i18next'
import type { Store } from '@/lib/types'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import { Button } from '@/components/ui/button'
import { Check, Database, Loader2, Pencil, RefreshCw, Star, Trash2 } from 'lucide-react'
import { formatBytes } from './stores'

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

// StoreCard renders a single store card: provider badges, quota usage, and
// the action buttons (edit / connect / set primary / test / ingest / delete).
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

  const providerLabel = (p: string) => {
    if (p === 'local') return t('stores.providerLocal')
    if (p === 's3') return t('stores.providerS3')
    if (p === 'gdrive') return t('stores.providerGdrive')
    return p
  }

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-base">
          <Database className="h-4 w-4 text-muted-foreground" />
          <span className="truncate">{store.name}</span>
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
        </CardTitle>
        <CardDescription>
          <Badge variant="secondary">{providerLabel(store.provider)}</Badge>
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3 text-sm">
        <dl className="grid grid-cols-2 gap-2 text-xs">
          <div>
            <dt className="text-muted-foreground">{t('stores.status')}</dt>
            <dd className="capitalize">{store.status}</dd>
          </div>
          <div>
            <dt className="text-muted-foreground">{t('stores.writeMode')}</dt>
            <dd>{store.write_mode}</dd>
          </div>
          <div>
            <dt className="text-muted-foreground">{t('stores.ingestMode')}</dt>
            <dd>{store.ingest_mode}</dd>
          </div>
          <div>
            <dt className="text-muted-foreground">{t('stores.readPriority')}</dt>
            <dd>{store.read_priority}</dd>
          </div>
          <div>
            <dt className="text-muted-foreground">{t('stores.storageLimit')}</dt>
            <dd>{store.quota_limit > 0 ? formatBytes(store.quota_limit) : t('stores.unlimited')}</dd>
          </div>
          {store.provider === 'local' && !!store.config?.baseDir && (
            <div className="col-span-2">
              <dt className="text-muted-foreground">{t('stores.folderLocation')}</dt>
              <dd className="font-mono text-[11px] break-all">{String(store.config.baseDir)}</dd>
            </div>
          )}
        </dl>

        {store.quota_limit > 0 && store.status === 'active' && (
          <div className="space-y-1">
            <Progress
              value={store.quota_limit > 0 ? Math.min(100, (store.quota_used / store.quota_limit) * 100) : 0}
              className="h-2"
            />
            {store.quota_limit > 0 && (
              <p className="text-xs text-muted-foreground">
                {t('stores.quotaUsage', {
                  used: formatBytes(store.quota_used),
                  limit: formatBytes(store.quota_limit),
                })}
              </p>
            )}
          </div>
        )}

        <div className="flex flex-wrap gap-2">
          <Button variant="outline" size="sm" onClick={() => onEdit(store)}>
            <Pencil className="h-3 w-3 mr-1" />
            {t('stores.editStore')}
          </Button>
          {store.provider === 'gdrive' && store.status !== 'active' && (
            <Button variant="outline" size="sm" disabled={gdriveAuthPending} onClick={() => onGdriveConnect(store.id)}>
              {gdriveAuthPending ? <Loader2 className="h-3 w-3 animate-spin" /> : <RefreshCw className="h-3 w-3 mr-1" />}
              {t('stores.connectGoogle')}
            </Button>
          )}
          {store.id !== primaryStoreId && (
            <Button variant="outline" size="sm" disabled={setPrimaryPending} onClick={() => onSetPrimary(store.id)}>
              <Star className="h-3 w-3 mr-1" />
              {t('stores.setPrimary')}
            </Button>
          )}
          <Button variant="outline" size="sm" disabled={testPending} onClick={() => onTest(store.id)}>
            {testPending ? <Loader2 className="h-3 w-3 animate-spin" /> : <Check className="h-3 w-3 mr-1" />}
            {t('stores.testConnection')}
          </Button>
          <Button variant="outline" size="sm" disabled={ingestPending} onClick={() => onIngest(store.id)}>
            {t('stores.triggerIngest')}
          </Button>
          <Button variant="ghost" size="sm" className="text-destructive" onClick={() => onDelete(store)}>
            <Trash2 className="h-3 w-3 mr-1" />
            {t('stores.deleteStore')}
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
