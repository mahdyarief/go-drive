import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { tenantApi } from '@/lib/api'
import type { BreadcrumbItem, Folder, LockerFile, ShareLink, StorageUsage, Tag } from '@/lib/types'
import { useOrgStore } from '@/store/org'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import {
  Breadcrumb,
  BreadcrumbItem as BCItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  Check,
  Copy,
  Download,
  Eye,
  File as FileIcon,
  Folder as FolderIcon,
  FolderInput,
  HardDrive,
  LayoutGrid,
  List,
  Loader2,
  MoreVertical,
  Pencil,
  Plus,
  Share2,
  Tag as TagIcon,
  Trash2,
  Upload,
} from 'lucide-react'

interface FileListData {
  files: LockerFile[]
  tags?: Record<string, Tag[]>
  total: number
  page: number
  pageSize: number
}

interface FolderListData {
  folders: Folder[]
}

interface BreadcrumbsData {
  breadcrumbs: BreadcrumbItem[]
}

interface DownloadUrlData {
  url: string
}

interface TagsData {
  tags: Tag[]
}

type ViewMode = 'list' | 'grid'

const VIEW_MODE_KEY = 'filesViewMode'

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`
}

interface ItemField {
  id: string
  name: string
  isFolder: boolean
  file?: LockerFile
}

const copyToClipboard = async (text: string) => {
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    // Clipboard access can be blocked in embedded contexts; ignore.
  }
}

export default function FilesPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const currentOrg = useOrgStore((s) => s.currentOrg)
  const orgSlug = currentOrg?.slug

  const [currentFolderId, setCurrentFolderId] = useState<string | null>(null)
  const [viewMode, setViewMode] = useState<ViewMode>(() => {
    const saved = localStorage.getItem(VIEW_MODE_KEY)
    return saved === 'grid' ? 'grid' : 'list'
  })
  const [renameTarget, setRenameTarget] = useState<ItemField | null>(null)
  const [renameValue, setRenameValue] = useState('')
  const [moveTarget, setMoveTarget] = useState<ItemField | null>(null)
  const [moveFolderId, setMoveFolderId] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<ItemField | null>(null)
  const [createOpen, setCreateOpen] = useState(false)
  const [newFolderName, setNewFolderName] = useState('')
  const [uploadProgress, setUploadProgress] = useState<{ name: string; percent: number } | null>(null)
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; item: ItemField } | null>(null)
  const [tagTarget, setTagTarget] = useState<LockerFile | null>(null)
  const [selectedTagIds, setSelectedTagIds] = useState<string[]>([])
  const [newTagName, setNewTagName] = useState('')
  const [shareTarget, setShareTarget] = useState<ItemField | null>(null)
  const [copied, setCopied] = useState(false)

  const foldersQuery = useQuery({
    queryKey: ['t', 'folders', orgSlug, currentFolderId],
    queryFn: () =>
      tenantApi<FolderListData>(
        `/api/t/folders${currentFolderId ? `?parentId=${currentFolderId}` : ''}`,
        orgSlug!,
      ),
    enabled: !!orgSlug,
  })

  const filesQuery = useQuery({
    queryKey: ['t', 'files', orgSlug, currentFolderId],
    queryFn: () =>
      tenantApi<FileListData>(
        `/api/t/files${currentFolderId ? `?folderId=${currentFolderId}` : ''}`,
        orgSlug!,
      ),
    enabled: !!orgSlug,
  })

  const breadcrumbsQuery = useQuery({
    queryKey: ['t', 'breadcrumbs', orgSlug, currentFolderId],
    queryFn: () =>
      tenantApi<BreadcrumbsData>(
        `/api/t/folders/breadcrumbs${currentFolderId ? `?folderId=${currentFolderId}` : ''}`,
        orgSlug!,
      ),
    enabled: !!orgSlug,
  })

  const usageQuery = useQuery({
    queryKey: ['t', 'usage', orgSlug],
    queryFn: () => tenantApi<StorageUsage>('/api/t/storage/usage', orgSlug!),
    enabled: !!orgSlug,
  })

  const tagsQuery = useQuery({
    queryKey: ['t', 'tags', orgSlug],
    queryFn: () => tenantApi<TagsData>('/api/t/tags', orgSlug!),
    enabled: !!orgSlug && !!tagTarget,
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['t', 'files', orgSlug, currentFolderId] })
    queryClient.invalidateQueries({ queryKey: ['t', 'folders', orgSlug, currentFolderId] })
    queryClient.invalidateQueries({ queryKey: ['t', 'usage', orgSlug] })
  }

  const createFolder = useMutation({
    mutationFn: (name: string) =>
      tenantApi<{ folder: Folder }>('/api/t/folders', orgSlug!, {
        method: 'POST',
        body: JSON.stringify({ name, parentId: currentFolderId ?? undefined }),
      }),
    onSuccess: () => {
      invalidate()
      setCreateOpen(false)
      setNewFolderName('')
    },
  })

  const renameItem = useMutation({
    mutationFn: ({ item, name }: { item: ItemField; name: string }) =>
      tenantApi<unknown>(
        `/api/t/${item.isFolder ? 'folders' : 'files'}/${item.id}`,
        orgSlug!,
        { method: 'PATCH', body: JSON.stringify({ name }) },
      ),
    onSuccess: () => {
      invalidate()
      setRenameTarget(null)
    },
  })

  const moveItem = useMutation({
    mutationFn: ({ item, folderId }: { item: ItemField; folderId: string }) =>
      tenantApi<unknown>(
        `/api/t/${item.isFolder ? 'folders' : 'files'}/${item.id}`,
        orgSlug!,
        {
          method: 'PATCH',
          body: JSON.stringify(item.isFolder ? { parentId: folderId || null } : { folderId: folderId || null }),
        },
      ),
    onSuccess: () => {
      invalidate()
      setMoveTarget(null)
      setMoveFolderId('')
    },
  })

  const deleteItem = useMutation({
    mutationFn: (item: ItemField) =>
      tenantApi<unknown>(`/api/t/${item.isFolder ? 'folders' : 'files'}/${item.id}`, orgSlug!, {
        method: 'DELETE',
      }),
    onSuccess: () => {
      invalidate()
      setDeleteTarget(null)
    },
  })

  const uploadFiles = useMutation({
    mutationFn: (files: FileList) => {
      const form = new FormData()
      form.append('file', files[0])
      if (currentFolderId) form.append('folderId', currentFolderId)
      setUploadProgress({ name: files[0].name, percent: 0 })
      return tenantApi<unknown>('/api/t/upload', orgSlug!, { method: 'POST', body: form })
    },
    onSuccess: () => {
      invalidate()
      setUploadProgress(null)
    },
    onError: () => setUploadProgress(null),
  })

  const download = useMutation({
    mutationFn: (fileId: string) =>
      tenantApi<DownloadUrlData>(`/api/t/files/${fileId}/download-url`, orgSlug!),
    onSuccess: (data) => {
      window.open(data.url, '_blank', 'noopener,noreferrer')
    },
  })

  const createTag = useMutation({
    mutationFn: (name: string) =>
      tenantApi<{ tag: Tag }>('/api/t/tags', orgSlug!, {
        method: 'POST',
        body: JSON.stringify({ name }),
      }),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['t', 'tags', orgSlug] })
      setSelectedTagIds((prev) => [...prev, data.tag.id])
      setNewTagName('')
    },
  })

  const setFileTags = useMutation({
    mutationFn: ({ fileId, tagIds }: { fileId: string; tagIds: string[] }) =>
      tenantApi<unknown>('/api/t/tags/set-file-tags', orgSlug!, {
        method: 'POST',
        body: JSON.stringify({ fileId, tagIds }),
      }),
    onSuccess: () => {
      invalidate()
      setTagTarget(null)
    },
  })

  const createShareLink = useMutation({
    mutationFn: (item: ItemField) =>
      tenantApi<{ link: ShareLink }>('/api/t/share-links', orgSlug!, {
        method: 'POST',
        body: JSON.stringify(
          item.isFolder
            ? { folderId: item.id, access: 'download' }
            : { fileId: item.id, access: 'download' },
        ),
      }),
  })

  const folders = foldersQuery.data?.folders ?? []
  const files = filesQuery.data?.files ?? []
  const crumbs = breadcrumbsQuery.data?.breadcrumbs ?? []
  const fileTags = filesQuery.data?.tags ?? {}
  const allTags = tagsQuery.data?.tags ?? []

  const handleUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const selected = e.target.files
    if (selected && selected.length > 0) {
      uploadFiles.mutate(selected)
      e.target.value = ''
    }
  }

  const handleNav = (id: string | null) => {
    setCurrentFolderId(id)
  }

  const toggleView = (mode: ViewMode) => {
    setViewMode(mode)
    localStorage.setItem(VIEW_MODE_KEY, mode)
  }

  const handleContextMenu = (e: React.MouseEvent, item: ItemField) => {
    e.preventDefault()
    setContextMenu({ x: e.clientX, y: e.clientY, item })
  }

  const openTagDialog = (file: LockerFile) => {
    setSelectedTagIds((fileTags[file.id] ?? []).map((tag) => tag.id))
    setNewTagName('')
    setTagTarget(file)
  }

  const handleShare = (item: ItemField) => {
    createShareLink.reset()
    setCopied(false)
    setShareTarget(item)
    createShareLink.mutate(item)
  }

  const handleCopy = async (text: string) => {
    await copyToClipboard(text)
    setCopied(true)
  }

  const shareUrl =
    shareTarget && createShareLink.data?.link
      ? `${window.location.origin}/shared/${createShareLink.data.link.token}`
      : ''

  const renderItemActions = (item: ItemField) => (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" className="h-8 w-8" aria-label={t('files.open')}>
          <MoreVertical className="h-4 w-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {!item.isFolder && (
          <DropdownMenuItem
            onClick={() => navigate(`/app/files/preview/${item.id}`, { state: { file: item.file } })}
          >
            <Eye className="h-4 w-4 mr-2" />
            {t('files.preview')}
          </DropdownMenuItem>
        )}
        {!item.isFolder && (
          <DropdownMenuItem onClick={() => download.mutate(item.id)}>
            <Download className="h-4 w-4 mr-2" />
            {t('files.download')}
          </DropdownMenuItem>
        )}
        {!item.isFolder && (
          <DropdownMenuItem onClick={() => item.file && openTagDialog(item.file)}>
            <TagIcon className="h-4 w-4 mr-2" />
            {t('files.tags')}
          </DropdownMenuItem>
        )}
        <DropdownMenuItem onClick={() => handleShare(item)}>
          <Share2 className="h-4 w-4 mr-2" />
          {t('files.share')}
        </DropdownMenuItem>
        <DropdownMenuItem
          onClick={() => {
            setRenameTarget(item)
            setRenameValue(item.name)
          }}
        >
          <Pencil className="h-4 w-4 mr-2" />
          {t('files.rename')}
        </DropdownMenuItem>
        <DropdownMenuItem
          onClick={() => {
            setMoveTarget(item)
            setMoveFolderId('')
          }}
        >
          <FolderInput className="h-4 w-4 mr-2" />
          {t('files.move')}
        </DropdownMenuItem>
        <DropdownMenuItem variant="destructive" onClick={() => setDeleteTarget(item)}>
          <Trash2 className="h-4 w-4 mr-2" />
          {t('files.delete')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )

  const renderList = () => (
    <ul className="divide-y divide-border">
      {folders.map((folder) => (
        <li
          key={folder.id}
          className="flex items-center gap-3 py-2"
          onContextMenu={(e) => handleContextMenu(e, { id: folder.id, name: folder.name, isFolder: true })}
        >
          <button
            type="button"
            onClick={() => handleNav(folder.id)}
            className="flex flex-1 items-center gap-3 text-left min-w-0"
          >
            <FolderIcon className="h-4 w-4 shrink-0 text-muted-foreground" />
            <span className="truncate text-sm font-medium">{folder.name}</span>
          </button>
          {renderItemActions({ id: folder.id, name: folder.name, isFolder: true })}
        </li>
      ))}
      {files.map((file) => (
        <li
          key={file.id}
          className="flex items-center gap-3 py-2"
          onContextMenu={(e) => handleContextMenu(e, { id: file.id, name: file.name, isFolder: false, file })}
        >
          <button
            type="button"
            onClick={() => navigate(`/app/files/preview/${file.id}`, { state: { file } })}
            className="flex flex-1 items-center gap-3 text-left min-w-0"
          >
            <FileIcon className="h-4 w-4 shrink-0 text-muted-foreground" />
            <span className="truncate text-sm font-medium">{file.name}</span>
            <span className="ml-auto text-xs text-muted-foreground shrink-0">
              {formatBytes(file.size)}
            </span>
          </button>
          {renderItemActions({ id: file.id, name: file.name, isFolder: false, file })}
        </li>
      ))}
    </ul>
  )

  const renderGrid = () => (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
      {folders.map((folder) => (
        <button
          key={folder.id}
          type="button"
          onClick={() => handleNav(folder.id)}
          onContextMenu={(e) => handleContextMenu(e, { id: folder.id, name: folder.name, isFolder: true })}
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
          onClick={() => navigate(`/app/files/preview/${file.id}`, { state: { file } })}
          onContextMenu={(e) => handleContextMenu(e, { id: file.id, name: file.name, isFolder: false, file })}
          className="flex flex-col items-center gap-2 rounded-lg border p-4 hover:bg-accent"
        >
          <FileIcon className="h-8 w-8 text-muted-foreground" />
          <span className="w-full truncate text-center text-sm font-medium">{file.name}</span>
          <span className="text-xs text-muted-foreground">{formatBytes(file.size)}</span>
        </button>
      ))}
    </div>
  )

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t('files.title')}</h1>
          <p className="text-sm text-muted-foreground">{t('files.description')}</p>
        </div>
        <div className="flex items-center gap-2">
          <div className="flex items-center rounded-lg border p-0.5">
            <Button
              variant={viewMode === 'list' ? 'secondary' : 'ghost'}
              size="icon"
              className="h-8 w-8"
              aria-label={t('files.listView')}
              onClick={() => toggleView('list')}
            >
              <List className="h-4 w-4" />
            </Button>
            <Button
              variant={viewMode === 'grid' ? 'secondary' : 'ghost'}
              size="icon"
              className="h-8 w-8"
              aria-label={t('files.gridView')}
              onClick={() => toggleView('grid')}
            >
              <LayoutGrid className="h-4 w-4" />
            </Button>
          </div>
          <Button variant="outline" onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4 mr-2" />
            {t('files.newFolder')}
          </Button>
          <Button asChild>
            <label className="cursor-pointer flex items-center gap-1.5">
              <Upload className="h-4 w-4" />
              {t('files.upload')}
              <input
                type="file"
                className="hidden"
                onChange={handleUpload}
                disabled={uploadFiles.isPending}
              />
            </label>
          </Button>
        </div>
      </div>

      {uploadProgress && (
        <Card>
          <CardContent className="py-3 text-sm">
            <p className="text-muted-foreground">
              {t('files.uploadProgress', {
                name: uploadProgress.name,
                percent: uploadProgress.percent,
              })}
            </p>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader className="pb-3">
          <Breadcrumb>
            <BreadcrumbList>
              <BCItem>
                <BreadcrumbLink asChild>
                  <button type="button" onClick={() => handleNav(null)} className="text-sm">
                    {t('files.root')}
                  </button>
                </BreadcrumbLink>
              </BCItem>
              {crumbs.map((c) => (
                <BCItem key={c.id}>
                  <BreadcrumbSeparator />
                  <BreadcrumbPage className="text-sm">{c.name}</BreadcrumbPage>
                </BCItem>
              ))}
            </BreadcrumbList>
          </Breadcrumb>
        </CardHeader>
        <Separator />
        <CardContent className="pt-4">
          {(foldersQuery.isPending || filesQuery.isPending) && (
            <p className="text-sm text-muted-foreground py-8 text-center">...</p>
          )}
          {(foldersQuery.isError || filesQuery.isError) && (
            <p className="text-sm text-destructive py-8 text-center">{t('files.loadError')}</p>
          )}
          {!foldersQuery.isPending && !filesQuery.isPending && folders.length === 0 && files.length === 0 && (
            <div className="py-12 text-center">
              <p className="text-sm text-muted-foreground">{t('files.empty')}</p>
              <p className="text-xs text-muted-foreground mt-1">{t('files.emptyHint')}</p>
            </div>
          )}
          {viewMode === 'grid' ? renderGrid() : renderList()}
        </CardContent>
      </Card>

      {usageQuery.isSuccess && (
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center gap-2 text-base">
              <HardDrive className="h-4 w-4" />
              {t('files.storageUsed')}
            </CardTitle>
          </CardHeader>
          <CardContent className="text-sm">
            <p className="text-muted-foreground">
              {formatBytes(usageQuery.data.used)} / {formatBytes(usageQuery.data.limit)}
            </p>
            <div className="mt-2 h-2 w-full rounded-full bg-muted">
              <div
                className="h-2 rounded-full bg-primary"
                style={{ width: `${Math.min(usageQuery.data.percentage, 100)}%` }}
              />
            </div>
            <p className="mt-2 text-xs text-muted-foreground">
              {t('files.filesCount', { count: usageQuery.data.fileCount })} ·{' '}
              {t('files.foldersCount', { count: usageQuery.data.folderCount })}
            </p>
          </CardContent>
        </Card>
      )}

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('files.newFolder')}</DialogTitle>
            <DialogDescription>{t('files.folderName')}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="folder-name">{t('files.folderName')}</Label>
              <Input
                id="folder-name"
                value={newFolderName}
                onChange={(e) => setNewFolderName(e.target.value)}
                placeholder={t('files.folderNamePlaceholder')}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>
              {t('links.cancel')}
            </Button>
            <Button
              disabled={!newFolderName.trim() || createFolder.isPending}
              onClick={() => createFolder.mutate(newFolderName.trim())}
            >
              {t('files.createFolder')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!renameTarget} onOpenChange={(o) => !o && setRenameTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('files.renameTitle')}</DialogTitle>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="rename-name">{t('files.folderName')}</Label>
            <Input id="rename-name" value={renameValue} onChange={(e) => setRenameValue(e.target.value)} />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRenameTarget(null)}>
              {t('links.cancel')}
            </Button>
            <Button
              disabled={!renameValue.trim() || renameItem.isPending}
              onClick={() => renameTarget && renameItem.mutate({ item: renameTarget, name: renameValue.trim() })}
            >
              {t('links.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!moveTarget} onOpenChange={(o) => !o && setMoveTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('files.moveTitle')}</DialogTitle>
            <DialogDescription>{t('files.moveHint')}</DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="move-folder">{t('links.folderId')}</Label>
            <Input
              id="move-folder"
              value={moveFolderId}
              onChange={(e) => setMoveFolderId(e.target.value)}
              placeholder="folderId"
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setMoveTarget(null)}>
              {t('links.cancel')}
            </Button>
            <Button
              disabled={moveItem.isPending}
              onClick={() => moveTarget && moveItem.mutate({ item: moveTarget, folderId: moveFolderId })}
            >
              {t('links.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!tagTarget} onOpenChange={(o) => !o && setTagTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('files.manageTags')}</DialogTitle>
            <DialogDescription>{tagTarget?.name}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="max-h-56 space-y-2 overflow-y-auto">
              {allTags.length === 0 && (
                <p className="text-sm text-muted-foreground">{t('files.noTags')}</p>
              )}
              {allTags.map((tag) => (
                <label
                  key={tag.id}
                  className="flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-sm"
                >
                  <input
                    type="checkbox"
                    className="h-4 w-4"
                    checked={selectedTagIds.includes(tag.id)}
                    onChange={(e) => {
                      setSelectedTagIds((prev) =>
                        e.target.checked ? [...prev, tag.id] : prev.filter((id) => id !== tag.id),
                      )
                    }}
                  />
                  {tag.name}
                </label>
              ))}
            </div>
            <div className="flex items-center gap-2">
              <Input
                value={newTagName}
                onChange={(e) => setNewTagName(e.target.value)}
                placeholder={t('files.newTagPlaceholder')}
              />
              <Button
                variant="outline"
                disabled={!newTagName.trim() || createTag.isPending}
                onClick={() => createTag.mutate(newTagName.trim())}
              >
                <Plus className="h-4 w-4 mr-2" />
                {t('files.createTag')}
              </Button>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setTagTarget(null)}>
              {t('links.cancel')}
            </Button>
            <Button
              disabled={setFileTags.isPending}
              onClick={() => tagTarget && setFileTags.mutate({ fileId: tagTarget.id, tagIds: selectedTagIds })}
            >
              {setFileTags.isPending && <Loader2 className="h-4 w-4 animate-spin mr-2" />}
              {t('links.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={!!shareTarget}
        onOpenChange={(o) => {
          if (!o) {
            setShareTarget(null)
            setCopied(false)
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('files.shareTitle')}</DialogTitle>
            <DialogDescription>{t('files.shareHint')}</DialogDescription>
          </DialogHeader>
          {createShareLink.isPending ? (
            <p className="flex items-center gap-2 py-4 text-sm text-muted-foreground">
              <Loader2 className="h-4 w-4 animate-spin" />
              {t('app.loading')}
            </p>
          ) : createShareLink.isError ? (
            <p className="py-4 text-sm text-destructive">
              {createShareLink.error instanceof Error
                ? createShareLink.error.message
                : t('files.actionError')}
            </p>
          ) : createShareLink.data ? (
            <div className="space-y-2">
              <div className="flex items-center gap-2 rounded-lg border px-3 py-2">
                <span className="flex-1 truncate font-mono text-xs">{shareUrl}</span>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8"
                  aria-label={t('files.copyLink')}
                  onClick={() => handleCopy(shareUrl)}
                >
                  {copied ? (
                    <Check className="h-4 w-4 text-emerald-600" />
                  ) : (
                    <Copy className="h-4 w-4" />
                  )}
                </Button>
              </div>
              {copied && <p className="text-xs text-emerald-600">{t('files.copied')}</p>}
            </div>
          ) : null}
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setShareTarget(null)
                setCopied(false)
              }}
            >
              {t('links.cancel')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={!!deleteTarget} onOpenChange={(o) => !o && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('files.deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {deleteTarget
                ? deleteTarget.isFolder
                  ? t('files.deleteConfirmFolder', { name: deleteTarget.name })
                  : t('files.deleteConfirmFile', { name: deleteTarget.name })
                : ''}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => setDeleteTarget(null)}>
              {t('links.cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={deleteItem.isPending}
              onClick={() => deleteTarget && deleteItem.mutate(deleteTarget)}
            >
              {t('files.delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {contextMenu && (
        <>
          <button
            type="button"
            aria-label={t('links.cancel')}
            className="fixed inset-0 z-40 cursor-default"
            onClick={() => setContextMenu(null)}
            onContextMenu={(e) => {
              e.preventDefault()
              setContextMenu(null)
            }}
          />
          <div
            className="fixed z-50 w-56 rounded-lg border bg-popover p-1 shadow-md"
            style={{ left: contextMenu.x, top: contextMenu.y }}
          >
            {!contextMenu.item.isFolder && (
              <button
                type="button"
                className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm hover:bg-accent"
                onClick={() => {
                  navigate(`/app/files/preview/${contextMenu.item.id}`, {
                    state: { file: contextMenu.item.file },
                  })
                  setContextMenu(null)
                }}
              >
                <Eye className="h-4 w-4" />
                {t('files.preview')}
              </button>
            )}
            {!contextMenu.item.isFolder && (
              <button
                type="button"
                className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm hover:bg-accent"
                onClick={() => {
                  download.mutate(contextMenu.item.id)
                  setContextMenu(null)
                }}
              >
                <Download className="h-4 w-4" />
                {t('files.download')}
              </button>
            )}
            {!contextMenu.item.isFolder && (
              <button
                type="button"
                className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm hover:bg-accent"
                onClick={() => {
                  if (contextMenu.item.file) openTagDialog(contextMenu.item.file)
                  setContextMenu(null)
                }}
              >
                <TagIcon className="h-4 w-4" />
                {t('files.tags')}
              </button>
            )}
            <button
              type="button"
              className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm hover:bg-accent"
              onClick={() => {
                handleShare(contextMenu.item)
                setContextMenu(null)
              }}
            >
              <Share2 className="h-4 w-4" />
              {t('files.share')}
            </button>
            <button
              type="button"
              className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm hover:bg-accent"
              onClick={() => {
                setRenameTarget(contextMenu.item)
                setRenameValue(contextMenu.item.name)
                setContextMenu(null)
              }}
            >
              <Pencil className="h-4 w-4" />
              {t('files.rename')}
            </button>
            <button
              type="button"
              className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm hover:bg-accent"
              onClick={() => {
                setMoveTarget(contextMenu.item)
                setMoveFolderId('')
                setContextMenu(null)
              }}
            >
              <FolderInput className="h-4 w-4" />
              {t('files.move')}
            </button>
            <button
              type="button"
              className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm text-destructive hover:bg-accent"
              onClick={() => {
                setDeleteTarget(contextMenu.item)
                setContextMenu(null)
              }}
            >
              <Trash2 className="h-4 w-4" />
              {t('files.delete')}
            </button>
          </div>
        </>
      )}
    </div>
  )
}
