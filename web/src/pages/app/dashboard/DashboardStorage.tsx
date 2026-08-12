import { useQuery } from '@tanstack/react-query'
import { tenantApi } from '@/lib/api'
import type { StorageUsage } from '@/lib/types'
import { useOrgStore } from '@/store/org'
import { StorageBreakdownCard } from '@/pages/app/stores/StorageBreakdownCard'
import { StorageUsageCard } from '@/pages/app/files/StorageUsageCard'

// DashboardStorage shows the workspace quota bar next to the per-category
// breakdown. Both cards are tenant-scoped via the current org slug.
export function DashboardStorage() {
  const orgSlug = useOrgStore((s) => s.currentOrg?.slug)

  const usageQuery = useQuery({
    queryKey: ['t', 'storage', 'usage', orgSlug],
    queryFn: () => tenantApi<StorageUsage>('/api/t/storage/usage', orgSlug!),
    enabled: !!orgSlug,
  })

  if (usageQuery.isPending || usageQuery.isError || !usageQuery.data) {
    return <StorageBreakdownCard />
  }

  return (
    <div className="grid gap-4 md:grid-cols-2">
      <StorageUsageCard usage={usageQuery.data} />
      <StorageBreakdownCard />
    </div>
  )
}
