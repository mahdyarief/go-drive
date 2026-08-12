import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { useNavigate, useSearchParams } from 'react-router'
import { tenantApi } from '@/lib/api'
import type { Folder, LockerFile, ShareLink, StorageUsage, Tag } from '@/lib/types'
import { useOrgStore } from '@/store/org'
import { useUploadStore } from '@/store/upload'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { Breadcrumb, BreadcrumbItem as BCItem, BreadcrumbLink, BreadcrumbList, BreadcrumbPage, BreadcrumbSeparator } from '@/components/ui/breadcrumb'
import { X } from 'lucide-react'
import { VIEW_MODE_KEY, copyToClipboard } from './files/files'
import type {
  BreadcrumbsData,
  DownloadUrlData,
  FileListData,
  FolderListData,
  ItemActions,
  ItemField,
  SearchResultsData,
  TagsData,
  ViewMode,
} from './files/files'
import { FileDialogs } from './files/FileDialogs'
import { FileList } from './files/FileList'
import { FileToolbar } from './files/FileToolbar'
import { StorageUsageCard } from './files/StorageUsageCard'

export default function FilesPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const currentOrg = useOrgStore((s) => s.currentOrg)
  const orgSlug = currentOrg?.slug
  const addUpload = useUploadStore((s) => s.add)
  const updateUpload = useUploadStore((s) => s.update)

  const [searchParams] = useSearchParams()
  const initialQuery = searchParams.get('q') ?? ''
  const [currentFolderId, setCurrentFolderId] = useState<string | null>(null)
  const [searchInput, setSearchInput] = useState(initialQuery)
  const [activeSearch, setActiveSearch] = useState(initialQuery)
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

  const searchQuery = useQuery({
    queryKey: ['t', 'files', 'search', orgSlug, activeSearch],
    queryFn: () =>
      tenantApi<SearchResultsData>(
        `/api/t/files/search?q=${encodeURIComponent(activeSearch)}`,
        orgSlug!,
      ),
    enabled: !!orgSlug && activeSearch.trim().length > 0,
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
      const id = crypto.randomUUID()
      addUpload({ id, name: files[0].name, percent: 0, status: 'uploading' })
      return tenantApi<unknown>('/api/t/upload', orgSlug!, { method: 'POST', body: form })
        .then((res) => {
          updateUpload(id, { percent: 100, status: 'done' })
          return res
        })
        .catch((err) => {
          updateUpload(id, { status: 'error' })
          throw err
        })
    },
    onSuccess: () => {
      invalidate()
    },
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
  const isSearching = activeSearch.trim().length > 0
  const searchFiles = searchQuery.data?.files ?? []
  const searchTags = searchQuery.data?.tags ?? {}

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
    setSelectedTagIds(((isSearching ? searchTags : fileTags)[file.id] ?? []).map((tag) => tag.id))
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
        <FileToolbar
          viewMode={viewMode}
          onToggleView={toggleView}
          searchValue={searchInput}
          onSearchChange={setSearchInput}
          onSearch={() => setActiveSearch(searchInput.trim())}
          onNewFolder={() => setCreateOpen(true)}
          onUpload={handleUpload}
          uploadPending={uploadFiles.isPending}
        />
      </div>

      <Card>
        <CardHeader className="pb-3">
          <Breadcrumb>
            <BreadcrumbList>
              <BCItem>
                <BreadcrumbLink render={<button type="button" onClick={() => handleNav(null)} className="text-sm" />}>
                  {t('files.root')}
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
          {isSearching ? (
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <p className="text-sm text-muted-foreground">
                  {t('files.searchResults', { query: activeSearch, count: searchFiles.length })}
                </p>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => {
                    setActiveSearch('')
                    setSearchInput('')
                  }}
                >
                  <X className="h-4 w-4 mr-1" />
                  {t('files.clearSearch')}
                </Button>
              </div>
              {searchQuery.isPending && (
                <p className="text-sm text-muted-foreground py-8 text-center">...</p>
              )}
              {searchQuery.isError && (
                <p className="text-sm text-destructive py-8 text-center">{t('files.loadError')}</p>
              )}
              {searchQuery.isSuccess && searchFiles.length === 0 && (
                <p className="text-sm text-muted-foreground py-12 text-center">
                  {t('files.noSearchResults', { query: activeSearch })}
                </p>
              )}
              <FileList
                viewMode={viewMode}
                folders={[]}
                files={searchFiles}
                fileStores={{}}
                onOpenFolder={handleNav}
                onOpenFile={(file) => navigate(`/app/files/preview/${file.id}`, { state: { file } })}
                onContextMenu={handleContextMenu}
                actions={actions}
              />
            </div>
          ) : (
            <>
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
            </>
          )}
        </CardContent>
      </Card>

      {usageQuery.isSuccess && <StorageUsageCard usage={usageQuery.data} />}

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
        orgSlug={orgSlug ?? ''}
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
