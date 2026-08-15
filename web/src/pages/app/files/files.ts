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
  total: number
  page: number
  pageSize: number
}

export interface BreadcrumbsData {
  breadcrumbs: BreadcrumbItem[]
}

export interface DownloadUrlData {
  url: string
}

export interface TagsData {
  tags: Tag[]
  total: number
  page: number
  pageSize: number
}

export interface SearchResultsData {
  files: LockerFile[]
  tags?: Record<string, Tag[]>
}

// Search filters forwarded to GET /api/t/files/search. kind is a mime-type
// prefix (e.g. "image/"), sizes are bytes, dates are YYYY-MM-DD.
export interface SearchFilters {
  kind?: string
  minSize?: string
  maxSize?: string
  from?: string
  to?: string
}

export const buildSearchUrl = (q: string, filters: SearchFilters): string => {
  const params = new URLSearchParams({ q })
  if (filters.kind) params.set('kind', filters.kind)
  if (filters.minSize) params.set('minSize', filters.minSize)
  if (filters.maxSize) params.set('maxSize', filters.maxSize)
  if (filters.from) params.set('from', filters.from)
  if (filters.to) params.set('to', filters.to)
  return `/api/t/files/search?${params.toString()}`
}

export type ViewMode = 'list' | 'grid'

export const VIEW_MODE_KEY = 'filesViewMode'

// Folder color palette offered in the new-folder dialog and applied to folder
// tiles; the Folder model stores the chosen hex value.
export const FOLDER_COLORS = [
  '#f59e0b',
  '#ef4444',
  '#3b82f6',
  '#10b981',
  '#8b5cf6',
  '#ec4899',
  '#06b6d4',
  '#64748b',
] as const

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
  folder?: Folder
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
  onDetails: (item: ItemField) => void
  onPreview: (item: ItemField) => void
  onDownload: (item: ItemField) => void
  onTags: (item: ItemField) => void
  onShare: (item: ItemField) => void
  onRename: (item: ItemField) => void
  onMove: (item: ItemField) => void
  onDelete: (item: ItemField) => void
  onDeleteWithFiles?: (item: ItemField) => void
}
