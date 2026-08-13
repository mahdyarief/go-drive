import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { Store } from '@/lib/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ChevronDown, ChevronRight, Database } from 'lucide-react'
import { PROVIDER_ICONS, providerLabel } from './stores'
import { StoreCard } from './StoreCard'

interface StoreGroupsProps {
  stores: Store[]
  primaryStoreId: string | null
  onEdit: (store: Store) => void
  onGdriveConnect: (storeId: string) => void
  gdriveAuthPending: boolean
  onSetPrimary: (storeId: string) => void
  setPrimaryPending: boolean
  onTest: (storeId: string) => void
  testPending: boolean
  onIngest: (storeId: string) => void
  ingestPending: boolean
  onDelete: (store: Store) => void
}

const PROVIDER_ORDER = ['gdrive', 's3', 'local'] as const

// StoreGroups clusters stores by provider (gdrive / s3 / local) so pages with
// many storages stay navigable. Each group header shows the provider label and
// store count, and can be collapsed to shrink the list.
export function StoreGroups({
  stores,
  primaryStoreId,
  onEdit,
  onGdriveConnect,
  gdriveAuthPending,
  onSetPrimary,
  setPrimaryPending,
  onTest,
  testPending,
  onIngest,
  ingestPending,
  onDelete,
}: StoreGroupsProps) {
  const { t } = useTranslation()
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({})

  const groups = PROVIDER_ORDER.map((provider) => ({
    provider,
    items: stores.filter((s) => s.provider === provider),
  })).filter((g) => g.items.length > 0)

  if (groups.length === 0) return null

  const toggle = (provider: string) => {
    setCollapsed((prev) => ({ ...prev, [provider]: !prev[provider] }))
  }

  return (
    <div className="space-y-6">
      {groups.map(({ provider, items }) => {
        const isCollapsed = !!collapsed[provider]
        const Icon = PROVIDER_ICONS[provider as keyof typeof PROVIDER_ICONS] ?? Database
        return (
          <section key={provider} className="space-y-3">
            <Button
              variant="ghost"
              size="sm"
              className="gap-1.5 px-0 font-medium text-foreground"
              aria-expanded={!isCollapsed}
              onClick={() => toggle(provider)}
            >
              {isCollapsed ? (
                <ChevronRight className="h-4 w-4 text-muted-foreground" />
              ) : (
                <ChevronDown className="h-4 w-4 text-muted-foreground" />
              )}
              <Icon className="h-4 w-4 text-muted-foreground" />
              {providerLabel(t, provider)}
              <Badge variant="secondary" className="ml-1">
                {items.length}
              </Badge>
            </Button>
            {!isCollapsed && (
              <div className="grid grid-cols-1 gap-3">
                {items.map((store) => (
                  <StoreCard
                    key={store.id}
                    store={store}
                    primaryStoreId={primaryStoreId}
                    onEdit={onEdit}
                    onGdriveConnect={onGdriveConnect}
                    gdriveAuthPending={gdriveAuthPending}
                    onSetPrimary={onSetPrimary}
                    setPrimaryPending={setPrimaryPending}
                    onTest={onTest}
                    testPending={testPending}
                    onIngest={onIngest}
                    ingestPending={ingestPending}
                    onDelete={onDelete}
                  />
                ))}
              </div>
            )}
          </section>
        )
      })}
    </div>
  )
}
