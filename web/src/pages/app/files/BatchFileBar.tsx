import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
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
import { FolderInput, Trash2, X } from 'lucide-react'
import type { BatchFileOps } from './batchOps'
import { FolderPicker } from './FolderPicker'

interface BatchFileBarProps {
  orgSlug: string
  ops: BatchFileOps
}

// BatchFileBar is the select-mode bulk action bar: the selected count with
// Move/Delete actions, plus the batch move folder-picker and delete confirm
// dialogs.
export function BatchFileBar({ orgSlug, ops }: BatchFileBarProps) {
  const { t } = useTranslation()
  const count = ops.selectedIds.length

  return (
    <>
      {ops.selectMode && count > 0 && (
        <div className="flex flex-wrap items-center gap-2 rounded-lg border bg-muted/50 px-3 py-2">
          <p className="text-sm font-medium">{t('files.selectedCount', { count })}</p>
          <Button size="sm" variant="outline" disabled={ops.movePending} onClick={() => ops.setMoveOpen(true)}>
            <FolderInput className="h-4 w-4 mr-2" />
            {t('files.moveSelected')}
          </Button>
          <Button size="sm" variant="destructive" disabled={ops.deletePending} onClick={() => ops.setDeleteOpen(true)}>
            <Trash2 className="h-4 w-4 mr-2" />
            {t('files.deleteSelected')}
          </Button>
          <Button size="sm" variant="ghost" onClick={ops.clearSelection}>
            <X className="h-4 w-4 mr-2" />
            {t('files.cancelSelection')}
          </Button>
        </div>
      )}

      <Dialog open={ops.moveOpen} onOpenChange={ops.setMoveOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('files.batchMoveTitle')}</DialogTitle>
            <DialogDescription>{t('files.batchMoveHint', { count })}</DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <FolderPicker orgSlug={orgSlug} value={ops.moveFolderId} onChange={ops.setMoveFolderId} />
            {ops.moveError && <p className="text-sm text-destructive">{ops.moveError}</p>}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => ops.setMoveOpen(false)}>
              {t('links.cancel')}
            </Button>
            <Button disabled={ops.movePending} onClick={() => ops.moveFiles(ops.moveFolderId)}>
              {t('files.move')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={ops.deleteOpen} onOpenChange={ops.setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('files.batchDeleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('files.batchDeleteConfirm', { count })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          {ops.deleteError && <p className="text-sm text-destructive">{ops.deleteError}</p>}
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => ops.setDeleteOpen(false)}>
              {t('links.cancel')}
            </AlertDialogCancel>
            <AlertDialogAction variant="destructive" disabled={ops.deletePending} onClick={ops.deleteFiles}>
              {t('files.delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
