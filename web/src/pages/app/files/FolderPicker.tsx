import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { tenantApi } from '@/lib/api'
import type { Folder } from '@/lib/types'
import { ChevronRight, Folder as FolderIcon, FolderOpen, HardDrive, Loader2 } from 'lucide-react'
import type { FolderListData } from './files'

interface FolderPickerProps {
  orgSlug: string
  value: string
  onChange: (folderId: string) => void
}

interface FolderNodeProps {
  orgSlug: string
  folder: Folder
  value: string
  onChange: (folderId: string) => void
}

// FolderNode is one row of the lazy folder tree: children are fetched only
// when the row is expanded, so deep trees stay cheap to browse.
function FolderNode({ orgSlug, folder, value, onChange }: FolderNodeProps) {
  const { t } = useTranslation()
  const [expanded, setExpanded] = useState(false)

  const childrenQuery = useQuery({
    queryKey: ['t', 'folders', orgSlug, folder.id],
    queryFn: () => tenantApi<FolderListData>(`/api/t/folders?parentId=${folder.id}`, orgSlug),
    enabled: expanded,
  })

  const children = childrenQuery.data?.folders ?? []
  const selected = value === folder.id

  return (
    <div>
      <div className="flex items-center gap-1">
        <button
          type="button"
          className="flex h-7 w-7 items-center justify-center rounded-md hover:bg-accent"
          aria-label={t('files.expandFolder')}
          onClick={() => setExpanded((v) => !v)}
        >
          <ChevronRight className={`h-4 w-4 transition-transform ${expanded ? 'rotate-90' : ''}`} />
        </button>
        <button
          type="button"
          onClick={() => onChange(folder.id)}
          className={`flex flex-1 items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm hover:bg-accent ${
            selected ? 'bg-accent font-medium' : ''
          }`}
        >
          {expanded ? (
            <FolderOpen className="h-4 w-4 shrink-0 text-muted-foreground" />
          ) : (
            <FolderIcon className="h-4 w-4 shrink-0 text-muted-foreground" />
          )}
          <span className="truncate">{folder.name}</span>
        </button>
      </div>
      {expanded && (
        <div className="ml-3 border-l pl-2">
          {childrenQuery.isPending && (
            <p className="flex items-center gap-2 px-2 py-1 text-xs text-muted-foreground">
              <Loader2 className="h-3 w-3 animate-spin" />
              ...
            </p>
          )}
          {children.map((child) => (
            <FolderNode key={child.id} orgSlug={orgSlug} folder={child} value={value} onChange={onChange} />
          ))}
        </div>
      )}
    </div>
  )
}

// FolderPicker is a lazy folder tree for the move dialog: the root row plus
// every folder the user expands. Selecting a folder reports its id; selecting
// the root row reports ''.
export function FolderPicker({ orgSlug, value, onChange }: FolderPickerProps) {
  const { t } = useTranslation()

  const rootQuery = useQuery({
    queryKey: ['t', 'folders', orgSlug, 'root'],
    queryFn: () => tenantApi<FolderListData>('/api/t/folders', orgSlug),
    enabled: !!orgSlug,
  })

  const folders = rootQuery.data?.folders ?? []

  return (
    <div className="max-h-64 space-y-1 overflow-y-auto rounded-lg border p-2">
      <button
        type="button"
        onClick={() => onChange('')}
        className={`flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm hover:bg-accent ${
          value === '' ? 'bg-accent font-medium' : ''
        }`}
      >
        <HardDrive className="h-4 w-4 shrink-0 text-muted-foreground" />
        {t('files.root')}
      </button>
      {rootQuery.isPending && (
        <p className="flex items-center gap-2 px-2 py-1 text-xs text-muted-foreground">
          <Loader2 className="h-3 w-3 animate-spin" />
          ...
        </p>
      )}
      {folders.map((folder) => (
        <FolderNode key={folder.id} orgSlug={orgSlug} folder={folder} value={value} onChange={onChange} />
      ))}
    </div>
  )
}
