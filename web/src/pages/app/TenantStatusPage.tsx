import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { tenantApi } from '@/lib/api'
import type { TenantStatusData } from '@/lib/types'
import { useOrgStore } from '@/store/org'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'

export default function TenantStatusPage() {
  const { t } = useTranslation()
  const currentOrg = useOrgStore((s) => s.currentOrg)

  const status = useQuery({
    queryKey: ['tenant', 'status', currentOrg?.slug],
    queryFn: () => tenantApi<TenantStatusData>('/api/t/status', currentOrg!.slug),
    enabled: !!currentOrg,
  })

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Tenant Status</h1>
        <p className="text-sm text-muted-foreground">{t('app.tenantStatusDescription')}</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            {t('app.tenantContext')}
            {status.isSuccess && <Badge variant="secondary">{status.data.schema}</Badge>}
          </CardTitle>
          <CardDescription>{t('app.tenantIsolationDemo')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          {status.isPending && <p className="text-muted-foreground">Loading...</p>}
          {status.isError && <p className="text-destructive">Failed to load tenant status</p>}
          {status.isSuccess && (
            <dl className="grid grid-cols-1 gap-2 sm:grid-cols-2">
              <div className="rounded-lg border p-3">
                <dt className="text-xs font-medium text-muted-foreground">Org ID</dt>
                <dd className="font-mono text-xs mt-1">{status.data.org_id}</dd>
              </div>
              <div className="rounded-lg border p-3">
                <dt className="text-xs font-medium text-muted-foreground">Org Slug</dt>
                <dd className="font-mono text-xs mt-1">{status.data.org_slug}</dd>
              </div>
              <div className="rounded-lg border p-3">
                <dt className="text-xs font-medium text-muted-foreground">Schema</dt>
                <dd className="font-mono text-xs mt-1">{status.data.schema}</dd>
              </div>
              <div className="rounded-lg border p-3">
                <dt className="text-xs font-medium text-muted-foreground">Role</dt>
                <dd className="font-mono text-xs mt-1">{status.data.role}</dd>
              </div>
            </dl>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
