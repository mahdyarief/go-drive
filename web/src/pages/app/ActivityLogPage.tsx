import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { History } from 'lucide-react'
import { tenantApi } from '@/lib/api'
import { useOrgStore } from '@/store/org'
import type { AuditLogsData } from './activity/activity'
import { ActivityLogRow } from './activity/ActivityLogRow'

const SKELETON_ROWS = 5

export default function ActivityLogPage() {
  const { t } = useTranslation()
  const orgSlug = useOrgStore((s) => s.currentOrg?.slug)

  const query = useQuery({
    queryKey: ['t', 'audit-logs', orgSlug],
    queryFn: () => tenantApi<AuditLogsData>('/api/t/audit-logs', orgSlug!),
    enabled: !!orgSlug,
  })

  const logs = query.data?.logs ?? []

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t('activity.title')}</h1>
        <p className="text-sm text-muted-foreground">{t('activity.subtitle')}</p>
      </div>

      {query.isPending ? (
        <ul className="divide-y divide-border rounded-lg border border-border">
          {Array.from({ length: SKELETON_ROWS }, (_, i) => (
            <li key={i} className="flex items-center gap-3 px-4 py-3">
              <div className="h-8 w-8 shrink-0 animate-pulse rounded-full bg-muted" />
              <div className="flex-1 space-y-2">
                <div className="h-3 w-1/3 animate-pulse rounded bg-muted" />
                <div className="h-3 w-1/2 animate-pulse rounded bg-muted" />
              </div>
            </li>
          ))}
        </ul>
      ) : query.isError ? (
        <div className="rounded-lg border border-dashed p-8 text-center">
          <p className="text-sm text-muted-foreground">{t('activity.error')}</p>
        </div>
      ) : logs.length === 0 ? (
        <div className="rounded-lg border border-dashed p-8 text-center">
          <History className="mx-auto mb-2 h-8 w-8 text-muted-foreground" />
          <p className="text-sm font-medium">{t('activity.empty')}</p>
          <p className="mt-1 text-sm text-muted-foreground">{t('activity.emptyHint')}</p>
        </div>
      ) : (
        <div>
          <h2 className="mb-2 px-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            {t('activity.recentTrail')}
          </h2>
          <ul className="divide-y divide-border rounded-lg border border-border">
            {logs.map((log) => (
              <ActivityLogRow key={log.id} log={log} />
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}
