import type { FileStoreInfo, Folder, LockerFile } from '@/lib/types'
import { Badge } from '@/components/ui/badge'
import { File as FileIcon, Folder as FolderIcon } from 'lucide-react'
import { FileItemActions } from './FileItemActions'
import { formatBytes, type ItemActions, type ItemField, type ViewMode } from './files'

interface FileListProps {
  viewMode: ViewMode
  folders: Folder[]
  files: LockerFile[]
  fileStores: Record<string, FileStoreInfo[]>
  onOpenFolder: (id: string) => void
  onOpenFile: (file: LockerFile) => void
  onContextMenu: (e: React.MouseEvent, item: ItemField) => void
  actions: ItemActions
}

const folderItem = (folder: Folder): ItemField => ({ id: folder.id, name: folder.name, isFolder: true })
const fileItem = (file: LockerFile): ItemField => ({ id: file.id, name: file.name, isFolder: false, file })

// FileList renders the current folder's contents as either a compact list or
// a grid, depending on viewMode. Rows/tiles open on click and expose the item
// actions dropdown; right-click opens the page-level context menu.
export function FileList({
  viewMode,
  folders,
  files,
  fileStores,
  onOpenFolder,
  onOpenFile,
  onContextMenu,
  actions,
}: FileListProps) {
  if (viewMode === 'grid') {
    return (
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
        {folders.map((folder) => (
          <button
            key={folder.id}
            type="button"
            onClick={() => onOpenFolder(folder.id)}
            onContextMenu={(e) => onContextMenu(e, folderItem(folder))}
            className="flex flex-col items-center gap-2 rounded-lg border p-4 hover:bg-accent"
          >
            <FolderIcon className="h-8 w-8 text-muted-foreground" />
            <span className="w-full truncate text-center text-sm font-medium">{folder.name}</span>
          </button>
        ))}
        {files.map((file) => (
          <button
            key={file.id}
            type="button"
            onClick={() => onOpenFile(file)}
            onContextMenu={(e) => onContextMenu(e, fileItem(file))}
            className="flex flex-col items-center gap-2 rounded-lg border p-4 hover:bg-accent"
          >
            <FileIcon className="h-8 w-8 text-muted-foreground" />
            <span className="w-full truncate text-center text-sm font-medium">{file.name}</span>
            <span className="text-xs text-muted-foreground">{formatBytes(file.size)}</span>
            {(fileStores[file.id] ?? []).slice(0, 1).map((s) => (
              <span key={s.id} className="text-[10px] text-muted-foreground">
                {s.name}
              </span>
            ))}
          </button>
        ))}
      </div>
    )
  }

  return (
    <ul className="divide-y divide-border">
      {folders.map((folder) => (
        <li
          key={folder.id}
          className="flex items-center gap-3 py-2"
          onContextMenu={(e) => onContextMenu(e, folderItem(folder))}
        >
          <button
            type="button"
            onClick={() => onOpenFolder(folder.id)}
            className="flex flex-1 items-center gap-3 text-left min-w-0"
          >
            <FolderIcon className="h-4 w-4 shrink-0 text-muted-foreground" />
            <span className="truncate text-sm font-medium">{folder.name}</span>
          </button>
          <FileItemActions item={folderItem(folder)} actions={actions} />
        </li>
      ))}
      {files.map((file) => (
        <li
          key={file.id}
          className="flex items-center gap-3 py-2"
          onContextMenu={(e) => onContextMenu(e, fileItem(file))}
        >
          <button
            type="button"
            onClick={() => onOpenFile(file)}
            className="flex flex-1 items-center gap-3 text-left min-w-0"
          >
            <FileIcon className="h-4 w-4 shrink-0 text-muted-foreground" />
            <span className="truncate text-sm font-medium">{file.name}</span>
            {(fileStores[file.id] ?? []).map((s) => (
              <Badge key={s.id} variant="secondary" className="shrink-0 text-[10px]">
                {s.name}
              </Badge>
            ))}
            <span className="ml-auto text-xs text-muted-foreground shrink-0">
              {formatBytes(file.size)}
            </span>
          </button>
          <FileItemActions item={fileItem(file)} actions={actions} />
        </li>
      ))}
    </ul>
  )
}
