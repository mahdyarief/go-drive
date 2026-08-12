import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Eye, Pencil, Trash2 } from 'lucide-react'
import type { AnyLink } from './links'

interface LinkRowProps {
  link: AnyLink
  children: ReactNode
  onEdit: () => void
  onDelete: () => void
  onShowEvents?: () => void
}

export function LinkRow({ link, children, onEdit, onDelete, onShowEvents }: LinkRowProps) {
  const { t } = useTranslation()
  return (
    <div className="flex items-center gap-3 rounded-lg border p-3">
      <div className="min-w-0 flex-1">{children}</div>
      <Badge variant={link.is_active ? 'default' : 'secondary'}>
        {link.is_active ? t('links.active') : t('links.inactive')}
      </Badge>
      <div className="flex items-center gap-1">
        {onShowEvents && (
          <Button variant="ghost" size="icon" className="h-8 w-8" aria-label={t('links.events')} onClick={onShowEvents}>
            <Eye className="h-4 w-4" />
          </Button>
        )}
        <Button variant="ghost" size="icon" className="h-8 w-8" aria-label={t('links.edit')} onClick={onEdit}>
          <Pencil className="h-4 w-4" />
        </Button>
        <Button variant="ghost" size="icon" className="h-8 w-8 text-destructive" aria-label={t('links.delete')} onClick={onDelete}>
          <Trash2 className="h-4 w-4" />
        </Button>
      </div>
    </div>
  )
}
