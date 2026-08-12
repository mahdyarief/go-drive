import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Download, Eye, FolderInput, Info, MoreVertical, Pencil, Share2, Tag as TagIcon, Trash2 } from 'lucide-react'
import type { ItemActions, ItemField } from './files'

interface FileItemActionsProps {
  item: ItemField
  actions: ItemActions
}

// FileItemActions renders the per-row "more" dropdown menu for a file or
// folder (preview / download / tags / share / rename / move / delete).
export function FileItemActions({ item, actions }: FileItemActionsProps) {
  const { t } = useTranslation()

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={<Button variant="ghost" size="icon" className="h-8 w-8" aria-label={t('files.open')} />}
      >
        <MoreVertical className="h-4 w-4" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {!item.isFolder && (
          <DropdownMenuItem onClick={() => actions.onPreview(item)}>
            <Eye className="h-4 w-4 mr-2" />
            {t('files.preview')}
          </DropdownMenuItem>
        )}
        {!item.isFolder && (
          <DropdownMenuItem onClick={() => actions.onDownload(item)}>
            <Download className="h-4 w-4 mr-2" />
            {t('files.download')}
          </DropdownMenuItem>
        )}
        {!item.isFolder && (
          <DropdownMenuItem onClick={() => actions.onTags(item)}>
            <TagIcon className="h-4 w-4 mr-2" />
            {t('files.tags')}
          </DropdownMenuItem>
        )}
        <DropdownMenuItem onClick={() => actions.onDetails(item)}>
          <Info className="h-4 w-4 mr-2" />
          {t('files.details')}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => actions.onShare(item)}>
          <Share2 className="h-4 w-4 mr-2" />
          {t('files.share')}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => actions.onRename(item)}>
          <Pencil className="h-4 w-4 mr-2" />
          {t('files.rename')}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => actions.onMove(item)}>
          <FolderInput className="h-4 w-4 mr-2" />
          {t('files.move')}
        </DropdownMenuItem>
        <DropdownMenuItem variant="destructive" onClick={() => actions.onDelete(item)}>
          <Trash2 className="h-4 w-4 mr-2" />
          {t('files.delete')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
