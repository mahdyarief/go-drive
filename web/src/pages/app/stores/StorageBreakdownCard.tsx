import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { PieChart } from 'lucide-react'
import { tenantApi } from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { formatBytes, useOrgSlug } from './stores'

interface StorageBreakdownData {
  breakdown: {
    photo: number
    video: number
    document: number
    total: number
  }
}

interface BreakdownSegment {
  key: 'photo' | 'video' | 'document'
  label: string
  bytes: number
  className: string
}

// StorageBreakdownCard fetches the tenant's storage usage grouped by category
// (photo/video/document) and renders a segmented bar plus per-category totals.
export function StorageBreakdownCard() {
  const { t } = useTranslation()
  const orgSlug = useOrgSlug()

  const breakdownQuery = useQuery({
    queryKey: ['t', 'storage', 'breakdown', orgSlug],
    queryFn: () => tenantApi<StorageBreakdownData>('/api/t/storage/breakdown', orgSlug!),
    enabled: !!orgSlug,
  })

  if (breakdownQuery.isPending) {
    return (
      <Card>
        <CardContent className="py-8 text-sm text-muted-foreground">{t('app.loading')}</CardContent>
      </Card>
    )
  }

  if (breakdownQuery.isError || !breakdownQuery.data) {
    return (
      <Card>
        <CardContent className="py-8 text-sm text-destructive">{t('storage.loadError')}</CardContent>
      </Card>
    )
  }

  const { photo, video, document: docs, total } = breakdownQuery.data.breakdown
  const segments: BreakdownSegment[] = [
    { key: 'photo', label: t('storage.photo'), bytes: photo, className: 'bg-sky-500' },
    { key: 'video', label: t('storage.video'), bytes: video, className: 'bg-violet-500' },
    { key: 'document', label: t('storage.document'), bytes: docs, className: 'bg-emerald-500' },
  ]

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-base">
          <PieChart className="h-4 w-4" />
          {t('storage.breakdownTitle')}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4 text-sm">
        <div className="flex h-2.5 w-full overflow-hidden rounded-full bg-muted">
          {segments.map((segment) => {
            const pct = total > 0 ? (segment.bytes / total) * 100 : 0
            if (pct <= 0) return null
            return <div key={segment.key} className={segment.className} style={{ width: `${pct}%` }} />
          })}
        </div>
        <div className="space-y-2">
          {segments.map((segment) => (
            <div key={segment.key} className="flex items-center justify-between">
              <span className="flex items-center gap-2 text-muted-foreground">
                <span className={`h-2.5 w-2.5 rounded-full ${segment.className}`} />
                {segment.label}
              </span>
              <span className="font-medium tabular-nums">{formatBytes(segment.bytes)}</span>
            </div>
          ))}
          <div className="flex items-center justify-between border-t pt-2">
            <span className="text-muted-foreground">{t('storage.total')}</span>
            <span className="font-medium tabular-nums">{formatBytes(total)}</span>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
