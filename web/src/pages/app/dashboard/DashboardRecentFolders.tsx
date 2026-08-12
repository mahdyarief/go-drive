import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { Folder } from 'lucide-react'
import { tenantApi } from '@/lib/api'
import { useOrgStore } from '@/store/org'
import { cn } from '@/lib/utils'
import { RECENT_FOLDERS_LIMIT, type RecentFoldersData } from './dashboard'

const FOLDER_COLOR_CLASSES: Record<string, string> = {
  yellow: 'bg-yellow-500/10 text-yellow-600 dark:text-yellow-400',
  blue: 'bg-sky-500/10 text-sky-600 dark:text-sky-400',
  green: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
  red: 'bg-rose-500/10 text-rose-600 dark:text-rose-400',
  purple: 'bg-violet-500/10 text-violet-600 dark:text-violet-400',
}

// DashboardRecentFolders lists the most recently updated folders as tappable
// tiles that deep-link into the files page for that folder.
export function DashboardRecentFolders() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const orgSlug = useOrgStore((s) => s.currentOrg?.slug)

  const query = useQuery({
    queryKey: ['t', 'folders', 'recent', orgSlug],
    queryFn: () => tenantApi<RecentFoldersData>(`/api/t/folders/recent?limit=${RECENT_FOLDERS_LIMIT}`, orgSlug!),
    enabled: !!orgSlug,
  })

  const folders = query.data?.folders ?? []

  return (
    <section>
      <h2 className="mb-2 px-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
        {t('dashboard.recentFolders')}
      </h2>
      {query.isPending ? (
        <div className="grid grid-cols-2 gap-2">
          {Array.from({ length: 4 }, (_, i) => (
            <div key={i} className="h-16 animate-pulse rounded-lg bg-muted" />
          ))}
        </div>
      ) : query.isError ? (
        <p className="text-sm text-destructive">{t('dashboard.error')}</p>
      ) : folders.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t('dashboard.emptyFolders')}</p>
      ) : (
        <div className="grid grid-cols-2 gap-2">
          {folders.map((folder) => (
            <button
              key={folder.id}
              type="button"
              onClick={() => navigate(`/app/files?folder=${folder.id}`)}
              className="flex items-center gap-2 rounded-lg border border-border bg-card p-2.5 text-left transition-colors hover:bg-accent"
              title={folder.name}
            >
              <span className={cn('flex h-8 w-8 shrink-0 items-center justify-center rounded-md', FOLDER_COLOR_CLASSES[folder.color] ?? FOLDER_COLOR_CLASSES.blue)}>
                <Folder className="h-4 w-4" />
              </span>
              <span className="truncate text-sm font-medium">{folder.name}</span>
            </button>
          ))}
        </div>
      )}
    </section>
  )
}
