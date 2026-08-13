import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { tenantApi } from '@/lib/api'
import { useOrgStore } from '@/store/org'
import type { AuditLogsData } from '@/pages/app/activity/activity'
import { ActivityLogRow } from '@/pages/app/activity/ActivityLogRow'
import { RECENT_ACTIVITY_LIMIT } from './dashboard'

// DashboardRecentActivity shows the latest audit log entries with a link to
// the full activity page.
export function DashboardRecentActivity() {
  const { t } = useTranslation()
  const orgSlug = useOrgStore((s) => s.currentOrg?.slug)

  const query = useQuery({
    queryKey: ['t', 'audit-logs', orgSlug],
    queryFn: () => tenantApi<AuditLogsData>('/api/t/audit-logs', orgSlug!),
    enabled: !!orgSlug,
  })

  const logs = (query.data?.logs ?? []).slice(0, RECENT_ACTIVITY_LIMIT)

  return (
    <section>
      <div className="mb-2 flex items-center justify-between px-1">
        <h2 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          {t('dashboard.recentActivity')}
        </h2>
        <Link to="/app/activity" className="text-xs font-medium text-primary hover:underline">
          {t('dashboard.viewAll')}
        </Link>
      </div>
      {query.isPending ? (
        <ul className="divide-y divide-border rounded-lg border border-border">
          {Array.from({ length: 3 }, (_, i) => (
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
        <p className="rounded-lg border border-border px-4 py-3 text-sm text-destructive">{t('dashboard.error')}</p>
      ) : logs.length === 0 ? (
        <p className="rounded-lg border border-border px-4 py-3 text-sm text-muted-foreground">{t('dashboard.emptyActivity')}</p>
      ) : (
        <ul className="divide-y divide-border rounded-lg border border-border">
          {logs.map((log) => (
            <ActivityLogRow key={log.id} log={log} />
          ))}
        </ul>
      )}
    </section>
  )
}
