import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { tenantApi } from '@/lib/api'
import type { ReplicationRun, StorageUsage, Store, TenantStatusData } from '@/lib/types'
import { useOrgStore } from '@/store/org'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { formatBytes } from '@/pages/app/files/files'
import { Activity, Database, HardDrive, RefreshCw } from 'lucide-react'

interface StoresData {
  stores: Store[]
}

interface SyncData {
  runs: ReplicationRun[]
}

// SystemStatusDropdown is the header's tenant health summary: schema, role,
// store count, storage usage, and the latest replication run. Data is fetched
// on first open and cached by TanStack Query afterwards.
export function SystemStatusDropdown() {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const currentOrg = useOrgStore((s) => s.currentOrg)
  const orgSlug = currentOrg?.slug

  const statusQuery = useQuery({
    queryKey: ['t', 'status', orgSlug],
    queryFn: () => tenantApi<TenantStatusData>('/api/t/status', orgSlug!),
    enabled: !!orgSlug && open,
  })

  const storesQuery = useQuery({
    queryKey: ['t', 'stores', orgSlug],
    queryFn: () => tenantApi<StoresData>('/api/t/stores', orgSlug!),
    enabled: !!orgSlug && open,
  })

  const usageQuery = useQuery({
    queryKey: ['t', 'usage', orgSlug],
    queryFn: () => tenantApi<StorageUsage>('/api/t/storage/usage', orgSlug!),
    enabled: !!orgSlug && open,
  })

  const syncQuery = useQuery({
    queryKey: ['t', 'stores', 'sync', orgSlug],
    queryFn: () => tenantApi<SyncData>('/api/t/stores/sync', orgSlug!),
    enabled: !!orgSlug && open,
  })

  const lastRun = syncQuery.data?.runs[0]

  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger
        render={<Button variant="ghost" size="icon" className="h-8 w-8" aria-label={t('app.systemStatus')} />}
      >
        <Activity className="h-4 w-4" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-72">
        <DropdownMenuLabel>{t('app.systemStatus')}</DropdownMenuLabel>
        {statusQuery.isSuccess && (
          <div className="flex items-center justify-between px-1.5 py-1 text-sm">
            <span className="text-muted-foreground">{t('app.tenantSchema')}</span>
            <Badge variant="secondary">{statusQuery.data.schema}</Badge>
          </div>
        )}
        {statusQuery.isSuccess && (
          <div className="flex items-center justify-between px-1.5 py-1 text-sm">
            <span className="text-muted-foreground">{t('app.tenantRole')}</span>
            <span>{statusQuery.data.role}</span>
          </div>
        )}
        <DropdownMenuSeparator />
        <div className="flex items-center justify-between px-1.5 py-1 text-sm">
          <span className="flex items-center gap-1.5 text-muted-foreground">
            <Database className="h-3.5 w-3.5" />
            {t('files.stores')}
          </span>
          <span>{storesQuery.isSuccess ? storesQuery.data.stores.length : '…'}</span>
        </div>
        {usageQuery.isSuccess && (
          <>
            <div className="flex items-center justify-between px-1.5 py-1 text-sm">
              <span className="flex items-center gap-1.5 text-muted-foreground">
                <HardDrive className="h-3.5 w-3.5" />
                {t('files.storageUsed')}
              </span>
              <span>
                {formatBytes(usageQuery.data.used)} /{' '}
                {usageQuery.data.limit > 0 ? formatBytes(usageQuery.data.limit) : '∞'}
              </span>
            </div>
            {usageQuery.data.limit > 0 && (
              <div className="px-1.5 pb-1.5">
                <div className="h-1.5 w-full rounded-full bg-muted">
                  <div
                    className="h-1.5 rounded-full bg-primary"
                    style={{ width: `${Math.min(usageQuery.data.percentage, 100)}%` }}
                  />
                </div>
              </div>
            )}
          </>
        )}
        <DropdownMenuSeparator />
        <div className="px-1.5 py-1 text-sm">
          <div className="flex items-center justify-between">
            <span className="flex items-center gap-1.5 text-muted-foreground">
              <RefreshCw className="h-3.5 w-3.5" />
              {t('app.lastSync')}
            </span>
            {lastRun ? (
              <Badge variant={lastRun.status === 'running' ? 'default' : 'secondary'}>{lastRun.status}</Badge>
            ) : syncQuery.isSuccess ? (
              <span className="text-xs text-muted-foreground">{t('app.noSyncRuns')}</span>
            ) : (
              <span>…</span>
            )}
          </div>
          {lastRun && (
            <p className="mt-1 text-xs text-muted-foreground">
              {lastRun.kind} · {lastRun.processed_items}/{lastRun.total_items}
            </p>
          )}
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
