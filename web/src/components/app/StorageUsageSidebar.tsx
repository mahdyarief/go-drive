import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import { tenantApi } from '@/lib/api'
import type { StorageUsage } from '@/lib/types'
import { useOrgStore } from '@/store/org'
import { formatBytes } from '@/pages/app/files/files'
import { HardDrive } from 'lucide-react'

interface StorageUsageSidebarProps {
  collapsed: boolean
}

// StorageUsageSidebar shows the tenant's storage quota bar at the bottom of
// the sidebar. When collapsed it renders only a thin bar; expanded it shows
// the used/total bytes with a label.
export function StorageUsageSidebar({ collapsed }: StorageUsageSidebarProps) {
  const { t } = useTranslation()
  const currentOrg = useOrgStore((s) => s.currentOrg)
  const orgSlug = currentOrg?.slug

  const usageQuery = useQuery({
    queryKey: ['t', 'usage', orgSlug],
    queryFn: () => tenantApi<StorageUsage>('/api/t/storage/usage', orgSlug!),
    enabled: !!orgSlug,
  })

  const usage = usageQuery.data
  if (!usage) return null

  const bar = (
    <div className="h-1.5 w-full rounded-full bg-muted" title={`${formatBytes(usage.used)} / ${formatBytes(usage.limit)}`}>
      <div className="h-1.5 rounded-full bg-primary" style={{ width: `${Math.min(usage.percentage, 100)}%` }} />
    </div>
  )

  if (collapsed) {
    return <div className="border-t border-border p-3">{bar}</div>
  }

  return (
    <div className="border-t border-border p-3">
      <div className="flex items-center gap-2 text-xs font-medium">
        <HardDrive className="h-3.5 w-3.5 text-muted-foreground" />
        <span className="text-muted-foreground">{t('files.storageUsed')}</span>
      </div>
      <p className="mt-1 text-xs text-muted-foreground">
        {formatBytes(usage.used)} / {formatBytes(usage.limit)}
      </p>
      <div className="mt-2">{bar}</div>
    </div>
  )
}
