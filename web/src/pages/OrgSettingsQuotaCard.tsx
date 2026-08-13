import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, tenantApi } from '@/lib/api'
import type { StorageUsage } from '@/lib/types'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { HardDrive } from 'lucide-react'
import { formatBytes } from '@/pages/app/stores/stores'

interface OrgSettingsQuotaCardProps {
  orgSlug: string
  quotaLimit: number
  isOwner: boolean
}

// OrgSettingsQuotaCard shows the org's allocated storage quota and lets the
// owner adjust the allocation up to their personal admin-assigned limit.
export function OrgSettingsQuotaCard({ orgSlug, quotaLimit, isOwner }: OrgSettingsQuotaCardProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [quotaGb, setQuotaGb] = useState(quotaLimit > 0 ? String(Math.round(quotaLimit / 1024 ** 3)) : '')

  const { data: usage, isLoading } = useQuery({
    queryKey: ['t', 'usage', orgSlug],
    queryFn: () => tenantApi<StorageUsage>('/api/t/storage/usage', orgSlug),
  })

  const setQuota = useMutation({
    mutationFn: (limit: number) =>
      api<void>(`/api/orgs/${orgSlug}/quota`, {
        method: 'PATCH',
        body: JSON.stringify({ limit }),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['t', 'usage', orgSlug] })
      queryClient.invalidateQueries({ queryKey: ['org', orgSlug] })
    },
  })

  const used = usage?.used ?? 0
  const allocated = quotaLimit > 0 ? quotaLimit : 0
  const percentage = allocated > 0 ? Math.min((used / allocated) * 100, 100) : 0

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-lg">
          <HardDrive className="h-4 w-4" />
          {t('org.storageQuota')}
        </CardTitle>
      </CardHeader>
      <Separator />
      <CardContent className="pt-4">
        {isLoading ? (
          <p className="text-sm text-muted-foreground">{t('app.loading')}</p>
        ) : (
          <>
            <p className="text-sm text-muted-foreground">
              {formatBytes(used)} /{' '}
              {allocated > 0 ? formatBytes(allocated) : t('org.quotaUnlimited')}
            </p>
            <div className="mt-2 h-2 w-full rounded-full bg-muted">
              <div
                className="h-2 rounded-full bg-primary"
                style={{ width: `${percentage}%` }}
              />
            </div>
            {isOwner ? (
              <div className="mt-4 space-y-3">
                <div className="space-y-2">
                  <Label htmlFor="org-quota-gb">{t('org.quotaGb')}</Label>
                  <Input
                    id="org-quota-gb"
                    type="number"
                    min={0}
                    step="any"
                    value={quotaGb}
                    onChange={(e) => setQuotaGb(e.target.value)}
                    placeholder={t('org.quotaUnlimitedHint')}
                  />
                </div>
                {setQuota.isError && (
                  <p className="text-sm text-destructive">{setQuota.error?.message}</p>
                )}
                <Button
                  disabled={setQuota.isPending}
                  onClick={() => {
                    const gb = Number(quotaGb)
                    if (!Number.isFinite(gb) || gb < 0) return
                    setQuota.mutate(Math.round(gb * 1024 ** 3))
                  }}
                >
                  {setQuota.isPending ? t('org.savingQuota') : t('org.setQuota')}
                </Button>
              </div>
            ) : (
              <p className="mt-3 text-xs text-muted-foreground">{t('org.quotaOwnerOnly')}</p>
            )}
          </>
        )}
      </CardContent>
    </Card>
  )
}
