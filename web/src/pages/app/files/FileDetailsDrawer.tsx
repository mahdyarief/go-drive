import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Separator } from '@/components/ui/separator'
import type { FileStoreInfo, Tag } from '@/lib/types'
import { Download, File as FileIcon, Folder as FolderIcon, FolderInput, Pencil, Share2, Trash2, X } from 'lucide-react'
import { formatBytes, type ItemActions, type ItemField } from './files'

interface FileDetailsDrawerProps {
  item: ItemField | null
  onClose: () => void
  stores: FileStoreInfo[]
  tags: Tag[]
  actions: ItemActions
}

const formatDate = (value?: string) => (value ? new Date(value).toLocaleString() : '—')

// FileDetailsDrawer is a right-side panel that shows a file's or folder's
// metadata (size, type, timestamps, storage, tags) with quick action buttons.
// It reuses the base-ui Dialog with right-edge positioning overrides.
export function FileDetailsDrawer({ item, onClose, stores, tags, actions }: FileDetailsDrawerProps) {
  const { t } = useTranslation()
  const file = item?.file
  const isFolder = item?.isFolder ?? false

  return (
    <Dialog open={!!item} onOpenChange={(o) => !o && onClose()}>
      <DialogContent
        showCloseButton={false}
        className="top-0 right-0 left-auto flex h-full w-full max-w-sm translate-x-0 translate-y-0 flex-col rounded-none sm:max-w-sm"
      >
        <DialogHeader className="flex-row items-center gap-3 pr-8">
          {isFolder ? (
            <FolderIcon className="h-6 w-6 shrink-0 text-muted-foreground" />
          ) : (
            <FileIcon className="h-6 w-6 shrink-0 text-muted-foreground" />
          )}
          <div className="min-w-0 flex-1">
            <DialogTitle className="truncate">{item?.name}</DialogTitle>
            <DialogDescription className="truncate">
              {isFolder ? t('files.folder') : file?.mime_type || t('files.type')}
            </DialogDescription>
          </div>
          <Button variant="ghost" size="icon" className="h-8 w-8" onClick={onClose} aria-label={t('files.dismiss')}>
            <X className="h-4 w-4" />
          </Button>
        </DialogHeader>

        <Separator />

        <dl className="flex-1 min-h-0 grid content-start gap-2 overflow-y-auto text-sm">
          {!isFolder && file && (
            <>
              <div className="flex items-center justify-between gap-4">
                <dt className="text-muted-foreground">{t('files.size')}</dt>
                <dd>{formatBytes(file.size)}</dd>
              </div>
              <div className="flex items-center justify-between gap-4">
                <dt className="text-muted-foreground">{t('files.storageProvider')}</dt>
                <dd className="truncate">{file.storage_provider || '—'}</dd>
              </div>
            </>
          )}
          <div className="flex items-center justify-between gap-4">
            <dt className="text-muted-foreground">{t('files.createdAt')}</dt>
            <dd>{formatDate(file?.created_at ?? item?.folder?.created_at)}</dd>
          </div>
          <div className="flex items-center justify-between gap-4">
            <dt className="text-muted-foreground">{t('files.updatedAt')}</dt>
            <dd>{formatDate(file?.updated_at ?? item?.folder?.updated_at)}</dd>
          </div>
          {!isFolder && stores.length > 0 && (
            <div className="flex items-center justify-between gap-4">
              <dt className="text-muted-foreground">{t('files.stores')}</dt>
              <dd className="flex flex-wrap justify-end gap-1">
                {stores.map((s) => (
                  <Badge key={s.id} variant="secondary" className="text-[10px]">
                    {s.name}
                  </Badge>
                ))}
              </dd>
            </div>
          )}
          {!isFolder && tags.length > 0 && (
            <div className="flex items-center justify-between gap-4">
              <dt className="text-muted-foreground">{t('files.tags')}</dt>
              <dd className="flex flex-wrap justify-end gap-1">
                {tags.map((tag) => (
                  <Badge key={tag.id} variant="secondary" className="text-[10px]">
                    {tag.name}
                  </Badge>
                ))}
              </dd>
            </div>
          )}
        </dl>

        <div className="grid shrink-0 gap-2">
          {!isFolder && (
            <Button variant="outline" onClick={() => item && actions.onDownload(item)}>
              <Download className="h-4 w-4 mr-2" />
              {t('files.download')}
            </Button>
          )}
          <div className="grid grid-cols-2 gap-2">
            <Button variant="outline" onClick={() => item && actions.onRename(item)}>
              <Pencil className="h-4 w-4 mr-2" />
              {t('files.rename')}
            </Button>
            <Button variant="outline" onClick={() => item && actions.onMove(item)}>
              <FolderInput className="h-4 w-4 mr-2" />
              {t('files.move')}
            </Button>
            <Button variant="outline" onClick={() => item && actions.onShare(item)}>
              <Share2 className="h-4 w-4 mr-2" />
              {t('files.share')}
            </Button>
            <Button variant="destructive" onClick={() => item && actions.onDelete(item)}>
              <Trash2 className="h-4 w-4 mr-2" />
              {t('files.delete')}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
