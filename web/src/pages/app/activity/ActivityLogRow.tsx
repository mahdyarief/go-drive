import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import type { AuditLog } from './activity'
import { actionLabelKey, badgeForAction, metadataParts } from './activity'

interface ActivityLogRowProps {
  log: AuditLog
}

export function ActivityLogRow({ log }: ActivityLogRowProps) {
  const { t } = useTranslation()
  const { className, Icon } = badgeForAction(log.action)
  const parts = metadataParts(log.metadata)

  return (
    <li className="flex items-start gap-3 px-4 py-3">
      <span className={cn('mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full', className)}>
        <Icon className="h-4 w-4" />
      </span>
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-baseline gap-x-2">
          <p className="text-sm font-medium">{t(actionLabelKey(log.action))}</p>
          <time className="text-xs text-muted-foreground" dateTime={log.createdAt}>
            {new Date(log.createdAt).toLocaleString()}
          </time>
        </div>
        {parts.length > 0 && (
          <p className="mt-0.5 truncate text-xs text-muted-foreground">
            {parts
              .map((p) => (p.kind === 'count' ? t('activity.metadataCount', { count: p.value }) : p.value))
              .join(' · ')}
          </p>
        )}
      </div>
    </li>
  )
}
