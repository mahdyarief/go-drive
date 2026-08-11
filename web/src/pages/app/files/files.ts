import type { BreadcrumbItem, FileStoreInfo, Folder, LockerFile, Tag } from '@/lib/types'

export interface FileListData {
  files: LockerFile[]
  tags?: Record<string, Tag[]>
  stores?: Record<string, FileStoreInfo[]>
  total: number
  page: number
  pageSize: number
}

export interface FolderListData {
  folders: Folder[]
}

export interface BreadcrumbsData {
  breadcrumbs: BreadcrumbItem[]
}

export interface DownloadUrlData {
  url: string
}

export interface TagsData {
  tags: Tag[]
}

export interface SearchResultsData {
  files: LockerFile[]
  tags?: Record<string, Tag[]>
}

export type ViewMode = 'list' | 'grid'

export const VIEW_MODE_KEY = 'filesViewMode'

export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`
}

export interface ItemField {
  id: string
  name: string
  isFolder: boolean
  file?: LockerFile
}

export const copyToClipboard = async (text: string) => {
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    // Clipboard access can be blocked in embedded contexts; ignore.
  }
}

// ItemActions bundles the handlers that both the per-row dropdown menu and the
// right-click context menu invoke for a file or folder.
export interface ItemActions {
  onPreview: (item: ItemField) => void
  onDownload: (item: ItemField) => void
  onTags: (item: ItemField) => void
  onShare: (item: ItemField) => void
  onRename: (item: ItemField) => void
  onMove: (item: ItemField) => void
  onDelete: (item: ItemField) => void
}
