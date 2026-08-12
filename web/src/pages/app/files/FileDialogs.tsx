import { useTranslation } from 'react-i18next'
import type { LockerFile, Tag } from '@/lib/types'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { cn } from '@/lib/utils'
import { FOLDER_COLORS } from './files'
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
import { Check, Copy, Download, Eye, FolderInput, Loader2, Pencil, Plus, Share2, Tag as TagIcon, Trash2 } from 'lucide-react'
import type { ItemActions, ItemField } from './files'
import { FolderPicker } from './FolderPicker'

interface FileDialogsProps {
  // New folder dialog
  createOpen: boolean
  setCreateOpen: (open: boolean) => void
  newFolderName: string
  setNewFolderName: (name: string) => void
  newFolderColor: string
  setNewFolderColor: (color: string) => void
  createFolderPending: boolean
  onCreateFolder: () => void
  // Rename dialog
  renameTarget: ItemField | null
  setRenameTarget: (target: ItemField | null) => void
  renameValue: string
  setRenameValue: (value: string) => void
  renameItemPending: boolean
  onRenameSubmit: () => void
  // Move dialog
  moveTarget: ItemField | null
  setMoveTarget: (target: ItemField | null) => void
  moveFolderId: string
  setMoveFolderId: (value: string) => void
  moveItemPending: boolean
  onMoveSubmit: () => void
  orgSlug: string
  // Tags dialog
  tagTarget: LockerFile | null
  setTagTarget: (target: LockerFile | null) => void
  selectedTagIds: string[]
  setSelectedTagIds: (ids: string[]) => void
  newTagName: string
  setNewTagName: (name: string) => void
  allTags: Tag[]
  createTagPending: boolean
  onCreateTag: () => void
  setFileTagsPending: boolean
  onSaveTags: () => void
  // Share dialog
  shareTarget: ItemField | null
  setShareTarget: (target: ItemField | null) => void
  copied: boolean
  setCopied: (copied: boolean) => void
  shareUrl: string
  sharePending: boolean
  shareErrorMessage: string | null
  onCopyShare: () => void
  // Delete confirm
  deleteTarget: ItemField | null
  setDeleteTarget: (target: ItemField | null) => void
  deleteItemPending: boolean
  onDeleteSubmit: () => void
  // Right-click context menu
  contextMenu: { x: number; y: number; item: ItemField } | null
  setContextMenu: (menu: { x: number; y: number; item: ItemField } | null) => void
  actions: ItemActions
}

// FileDialogs owns every modal and popover of the files page: new folder,
// rename, move, tags, share, delete confirm, and the right-click context menu.
export function FileDialogs(props: FileDialogsProps) {
  const { t } = useTranslation()
  const {
    createOpen, setCreateOpen, newFolderName, setNewFolderName, newFolderColor, setNewFolderColor, createFolderPending, onCreateFolder,
    renameTarget, setRenameTarget, renameValue, setRenameValue, renameItemPending, onRenameSubmit,
    moveTarget, setMoveTarget, moveFolderId, setMoveFolderId, moveItemPending, onMoveSubmit, orgSlug,
    tagTarget, setTagTarget, selectedTagIds, setSelectedTagIds, newTagName, setNewTagName,
    allTags, createTagPending, onCreateTag, setFileTagsPending, onSaveTags,
    shareTarget, setShareTarget, copied, setCopied, shareUrl, sharePending, shareErrorMessage, onCopyShare,
    deleteTarget, setDeleteTarget, deleteItemPending, onDeleteSubmit,
    contextMenu, setContextMenu, actions,
  } = props

  return (
    <>
      {/* New folder dialog */}
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
            <div className="space-y-2">
              <Label>{t('files.folderColor')}</Label>
              <div className="flex flex-wrap gap-2">
                {FOLDER_COLORS.map((color) => (
                  <button
                    key={color}
                    type="button"
                    aria-label={color}
                    onClick={() => setNewFolderColor(color)}
                    className={cn(
                      'h-6 w-6 rounded-full border-2 transition-colors',
                      newFolderColor === color ? 'border-foreground' : 'border-transparent',
                    )}
                    style={{ backgroundColor: color }}
                  />
                ))}
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>
              {t('links.cancel')}
            </Button>
            <Button disabled={!newFolderName.trim() || createFolderPending} onClick={onCreateFolder}>
              {t('files.createFolder')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Rename dialog */}
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
            <Button disabled={!renameValue.trim() || renameItemPending} onClick={onRenameSubmit}>
              {t('links.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Move dialog */}
      <Dialog open={!!moveTarget} onOpenChange={(o) => !o && setMoveTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('files.moveTitle')}</DialogTitle>
            <DialogDescription>{t('files.moveHint')}</DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label>{t('files.moveTo')}</Label>
            <FolderPicker orgSlug={orgSlug} value={moveFolderId} onChange={setMoveFolderId} />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setMoveTarget(null)}>
              {t('links.cancel')}
            </Button>
            <Button disabled={moveItemPending} onClick={onMoveSubmit}>
              {t('links.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Tags dialog */}
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
                      setSelectedTagIds(
                        e.target.checked ? [...selectedTagIds, tag.id] : selectedTagIds.filter((id) => id !== tag.id),
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
                disabled={!newTagName.trim() || createTagPending}
                onClick={onCreateTag}
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
            <Button disabled={setFileTagsPending} onClick={onSaveTags}>
              {setFileTagsPending && <Loader2 className="h-4 w-4 animate-spin mr-2" />}
              {t('links.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Share dialog */}
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
          {sharePending ? (
            <p className="flex items-center gap-2 py-4 text-sm text-muted-foreground">
              <Loader2 className="h-4 w-4 animate-spin" />
              {t('app.loading')}
            </p>
          ) : shareErrorMessage ? (
            <p className="py-4 text-sm text-destructive">{shareErrorMessage}</p>
          ) : shareUrl ? (
            <div className="space-y-2">
              <div className="flex items-center gap-2 rounded-lg border px-3 py-2">
                <span className="flex-1 truncate font-mono text-xs">{shareUrl}</span>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8"
                  aria-label={t('files.copyLink')}
                  onClick={onCopyShare}
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

      {/* Delete confirm */}
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
            <AlertDialogAction variant="destructive" disabled={deleteItemPending} onClick={onDeleteSubmit}>
              {t('files.delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Right-click context menu */}
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
                  actions.onPreview(contextMenu.item)
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
                  actions.onDownload(contextMenu.item)
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
                  actions.onTags(contextMenu.item)
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
                actions.onShare(contextMenu.item)
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
                actions.onRename(contextMenu.item)
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
                actions.onMove(contextMenu.item)
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
                actions.onDelete(contextMenu.item)
                setContextMenu(null)
              }}
            >
              <Trash2 className="h-4 w-4" />
              {t('files.delete')}
            </button>
          </div>
        </>
      )}
    </>
  )
}
