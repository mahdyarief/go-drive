import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { File } from 'lucide-react'
import { tenantApi } from '@/lib/api'
import { useOrgStore } from '@/store/org'
import { formatBytes } from '@/pages/app/files/files'
import { RECENT_FILES_LIMIT, type RecentFilesData } from './dashboard'

// DashboardRecentFiles shows the most recently updated files across all
// folders; clicking a row opens its preview page.
export function DashboardRecentFiles() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const orgSlug = useOrgStore((s) => s.currentOrg?.slug)

  const query = useQuery({
    queryKey: ['t', 'files', 'recent', orgSlug],
    queryFn: () => tenantApi<RecentFilesData>(`/api/t/files/recent?limit=${RECENT_FILES_LIMIT}`, orgSlug!),
    enabled: !!orgSlug,
  })

  const files = query.data?.files ?? []

  return (
    <section>
      <h2 className="mb-2 px-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
        {t('dashboard.recentFiles')}
      </h2>
      {query.isPending ? (
        <div className="space-y-2">
          {Array.from({ length: 3 }, (_, i) => (
            <div key={i} className="h-10 animate-pulse rounded-lg bg-muted" />
          ))}
        </div>
      ) : query.isError ? (
        <p className="text-sm text-destructive">{t('dashboard.error')}</p>
      ) : files.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t('dashboard.emptyFiles')}</p>
      ) : (
        <ul className="divide-y divide-border rounded-lg border border-border">
          {files.map((file) => (
            <li key={file.id}>
              <button
                type="button"
                onClick={() => navigate(`/app/files/preview/${file.id}`)}
                className="flex w-full items-center gap-3 px-3 py-2 text-left transition-colors hover:bg-accent"
                title={file.name}
              >
                <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
                  <File className="h-4 w-4" />
                </span>
                <span className="min-w-0 flex-1 truncate text-sm font-medium">{file.name}</span>
                <span className="shrink-0 text-xs text-muted-foreground tabular-nums">{formatBytes(file.size)}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
