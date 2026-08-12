import { useTranslation } from 'react-i18next'
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
import type { LinkKind } from './links'

interface DeleteLinkDialogProps {
  target: { kind: LinkKind; id: string } | null
  isPending: boolean
  onConfirm: () => void
  onClose: () => void
}

export function DeleteLinkDialog({ target, isPending, onConfirm, onClose }: DeleteLinkDialogProps) {
  const { t } = useTranslation()
  return (
    <AlertDialog open={!!target} onOpenChange={(o) => { if (!o) onClose() }}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('links.delete')}</AlertDialogTitle>
          <AlertDialogDescription>{t('links.deleteConfirm')}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel onClick={onClose}>{t('links.cancel')}</AlertDialogCancel>
          <AlertDialogAction variant="destructive" disabled={isPending} onClick={onConfirm}>
            {t('links.delete')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
