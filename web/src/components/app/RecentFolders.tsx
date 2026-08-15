import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router'
import { Folder } from 'lucide-react'
import { tenantApi } from '@/lib/api'
import type { Folder as FolderItem } from '@/lib/types'
import { useOrgStore } from '@/store/org'
import { cn } from '@/lib/utils'

interface RecentFoldersProps {
  collapsed: boolean
  onClose?: () => void
}

// RecentFolders lists the most recently updated folders as a quick-nav
// section in the sidebar. When collapsed, only the folder icons remain.
export function RecentFolders({ collapsed, onClose }: RecentFoldersProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const currentOrg = useOrgStore((s) => s.currentOrg)
  const orgSlug = currentOrg?.slug

  const recentQuery = useQuery({
    queryKey: ['t', 'folders', 'recent', orgSlug],
    queryFn: () => tenantApi<{ folders: FolderItem[] }>('/api/t/folders/recent?limit=4', orgSlug!),
    enabled: !!orgSlug,
  })

  const folders = recentQuery.data?.folders ?? []
  if (folders.length === 0) return null

  return (
    <div className="mt-4">
      <div
        className={cn(
          'px-3 text-xs font-semibold uppercase tracking-wide text-muted-foreground',
          collapsed && 'md:hidden',
        )}
      >
        {t('files.recentFolders')}
      </div>
      <div className="mt-1 space-y-1">
        {folders.map((folder) => (
          <button
            key={folder.id}
            type="button"
            title={folder.name}
            aria-label={folder.name}
            onClick={() => {
              onClose?.()
              navigate(`/app/files?folder=${folder.id}`)
            }}
            className={cn(
              'flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground',
              collapsed && 'md:justify-center md:px-2',
            )}
          >
            <Folder className="h-4 w-4 shrink-0" />
            <span className={cn('truncate', collapsed && 'md:hidden')}>{folder.name}</span>
          </button>
        ))}
      </div>
    </div>
  )
}
