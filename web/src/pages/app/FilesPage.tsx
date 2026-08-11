import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { tenantApi } from '@/lib/api'
import type { Folder, LockerFile, ShareLink, StorageUsage, Tag } from '@/lib/types'
import { useOrgStore } from '@/store/org'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { Breadcrumb, BreadcrumbItem as BCItem, BreadcrumbLink, BreadcrumbList, BreadcrumbPage, BreadcrumbSeparator } from '@/components/ui/breadcrumb'
import { HardDrive, LayoutGrid, List, Plus, Upload } from 'lucide-react'
import {
  BreadcrumbsData,
  DownloadUrlData,
  FileListData,
  FolderListData,
  ItemActions,
  ItemField,
  TagsData,
  ViewMode,
  VIEW_MODE_KEY,
  copyToClipboard,
  formatBytes,
} from './files/files'
import { FileDialogs } from './files/FileDialogs'
import { FileList } from './files/FileList'

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
  const fileStores = filesQuery.data?.stores ?? {}
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

  // Item actions shared by the per-row dropdown (FileItemActions) and the
  // right-click context menu (FileDialogs).
  const actions: ItemActions = {
    onPreview: (item) => navigate(`/app/files/preview/${item.id}`, { state: { file: item.file } }),
    onDownload: (item) => download.mutate(item.id),
    onTags: (item) => item.file && openTagDialog(item.file),
    onShare: (item) => handleShare(item),
    onRename: (item) => {
      setRenameTarget(item)
      setRenameValue(item.name)
    },
    onMove: (item) => {
      setMoveTarget(item)
      setMoveFolderId('')
    },
    onDelete: (item) => setDeleteTarget(item),
  }

  const shareErrorMessage = createShareLink.isError
    ? createShareLink.error instanceof Error
      ? createShareLink.error.message
      : t('files.actionError')
    : null

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
          <FileList
            viewMode={viewMode}
            folders={folders}
            files={files}
            fileStores={fileStores}
            onOpenFolder={handleNav}
            onOpenFile={(file) => navigate(`/app/files/preview/${file.id}`, { state: { file } })}
            onContextMenu={handleContextMenu}
            actions={actions}
          />
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

      <FileDialogs
        createOpen={createOpen}
        setCreateOpen={setCreateOpen}
        newFolderName={newFolderName}
        setNewFolderName={setNewFolderName}
        createFolderPending={createFolder.isPending}
        onCreateFolder={() => createFolder.mutate(newFolderName.trim())}
        renameTarget={renameTarget}
        setRenameTarget={setRenameTarget}
        renameValue={renameValue}
        setRenameValue={setRenameValue}
        renameItemPending={renameItem.isPending}
        onRenameSubmit={() => renameTarget && renameItem.mutate({ item: renameTarget, name: renameValue.trim() })}
        moveTarget={moveTarget}
        setMoveTarget={setMoveTarget}
        moveFolderId={moveFolderId}
        setMoveFolderId={setMoveFolderId}
        moveItemPending={moveItem.isPending}
        onMoveSubmit={() => moveTarget && moveItem.mutate({ item: moveTarget, folderId: moveFolderId })}
        tagTarget={tagTarget}
        setTagTarget={setTagTarget}
        selectedTagIds={selectedTagIds}
        setSelectedTagIds={setSelectedTagIds}
        newTagName={newTagName}
        setNewTagName={setNewTagName}
        allTags={allTags}
        createTagPending={createTag.isPending}
        onCreateTag={() => createTag.mutate(newTagName.trim())}
        setFileTagsPending={setFileTags.isPending}
        onSaveTags={() => tagTarget && setFileTags.mutate({ fileId: tagTarget.id, tagIds: selectedTagIds })}
        shareTarget={shareTarget}
        setShareTarget={setShareTarget}
        copied={copied}
        setCopied={setCopied}
        shareUrl={shareUrl}
        sharePending={createShareLink.isPending}
        shareErrorMessage={shareErrorMessage}
        onCopyShare={() => handleCopy(shareUrl)}
        deleteTarget={deleteTarget}
        setDeleteTarget={setDeleteTarget}
        deleteItemPending={deleteItem.isPending}
        onDeleteSubmit={() => deleteTarget && deleteItem.mutate(deleteTarget)}
        contextMenu={contextMenu}
        setContextMenu={setContextMenu}
        actions={actions}
      />
    </div>
  )
}
