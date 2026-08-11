import { useTranslation } from 'react-i18next'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { HardDrive } from 'lucide-react'
import type { StorageUsage } from '@/lib/types'
import { formatBytes } from './files'

interface StorageUsageCardProps {
  usage: StorageUsage
}

// StorageUsageCard renders the tenant's storage used/limit bar plus file and
// folder counts.
export function StorageUsageCard({ usage }: StorageUsageCardProps) {
  const { t } = useTranslation()

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-base">
          <HardDrive className="h-4 w-4" />
          {t('files.storageUsed')}
        </CardTitle>
      </CardHeader>
      <CardContent className="text-sm">
        <p className="text-muted-foreground">
          {formatBytes(usage.used)} / {formatBytes(usage.limit)}
        </p>
        <div className="mt-2 h-2 w-full rounded-full bg-muted">
          <div
            className="h-2 rounded-full bg-primary"
            style={{ width: `${Math.min(usage.percentage, 100)}%` }}
          />
        </div>
        <p className="mt-2 text-xs text-muted-foreground">
          {t('files.filesCount', { count: usage.fileCount })} ·{' '}
          {t('files.foldersCount', { count: usage.folderCount })}
        </p>
      </CardContent>
    </Card>
  )
}
