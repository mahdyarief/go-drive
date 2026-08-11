import { useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router'
import { tenantApi } from '@/lib/api'
import type { Store } from '@/lib/types'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Database, Plus, RefreshCw } from 'lucide-react'
import { toast } from 'sonner'
import { GDRIVE_CONNECTED_KEY, bytesToGB, useOrgSlug } from './stores/stores'
import type {
  StoreForm,
  StoresData,
  TestStoreData,
  IngestData,
  SyncData,
  TriggerSyncData,
  GDriveCompleteData,
  Provider,
  WriteMode,
  IngestMode,
} from './stores/stores'
import { StoreGroups } from './stores/StoreGroups'
import { S3KeysCard } from './stores/S3KeysCard'
import { StoreFormDialog } from './stores/StoreFormDialog'

export default function StoresPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const orgSlug = useOrgSlug()
  // S3 gateway endpoint for the connect guide. In development the app runs
  // on the Vite port (:5173), but the dev proxy rewrites the Host header
  // which breaks AWS SigV4 — so S3 clients must target the API port (:8081).
  const isDev = import.meta.env.DEV
  const serverBase = isDev ? 'http://localhost:8081' : window.location.origin
  const s3Endpoint = `${serverBase}/api/s3/${orgSlug ?? ''}`

  const [createOpen, setCreateOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<Store | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<Store | null>(null)
  const [showGdriveHelp, setShowGdriveHelp] = useState(false)
  const [showLocalHelp, setShowLocalHelp] = useState(false)

  // Create store form state
  const [form, setForm] = useState<StoreForm>({
    name: '',
    provider: 'local',
    writeMode: 'write',
    ingestMode: 'none',
    readPriority: 100,
    quotaLimit: 0,
    config: {},
    credentials: {},
  })

  const [searchParams, setSearchParams] = useSearchParams()
  const [gdriveError, setGdriveError] = useState(false)
  // Tracks OAuth states already handled so the complete call fires exactly
  // once per state (React StrictMode double-runs effects in dev).
  const handledGdriveStates = useRef(new Set<string>())

  // Handle the return from the Google consent screen: ?gdrive=connected&state=...
  useEffect(() => {
    const gdrive = searchParams.get('gdrive')
    if (!gdrive || !orgSlug) return
    const state = searchParams.get('state') ?? ''
    setSearchParams({}, { replace: true })
    if (gdrive === 'error') {
      setGdriveError(true)
      return
    }
    if (gdrive !== 'connected') return
    // StrictMode double-mounts effects in dev; skip the duplicate so the
    // complete endpoint is not called twice with the same state.
    if (handledGdriveStates.current.has(state)) return
    handledGdriveStates.current.add(state)
    tenantApi<GDriveCompleteData>(
      `/api/t/stores/gdrive/complete?state=${encodeURIComponent(state)}`,
      orgSlug,
    )
      .then((data) => {
        setGdriveError(false)
        // Notify other open tabs (the one that started the flow) so their
        // store list refreshes and shows the Connected badge.
        localStorage.setItem(
          GDRIVE_CONNECTED_KEY,
          JSON.stringify({ storeId: data.storeId, used: data.used, limit: data.limit }),
        )
        queryClient.invalidateQueries({ queryKey: ['t', 'stores', orgSlug] })
        queryClient.invalidateQueries({ queryKey: ['t', 'stores', 'sync', orgSlug] })
        toast.success(t('stores.testConnected'))
      })
      .catch(() => {
        setGdriveError(true)
      })
  }, [searchParams, setSearchParams, orgSlug, queryClient, t])

  // A connect that finishes in the OAuth tab writes to localStorage; the
  // storage event fires here (other tabs) so this tab refreshes too.
  useEffect(() => {
    const onStorage = (e: StorageEvent) => {
      if (e.key !== GDRIVE_CONNECTED_KEY || !e.newValue || !orgSlug) return
      queryClient.invalidateQueries({ queryKey: ['t', 'stores', orgSlug] })
      queryClient.invalidateQueries({ queryKey: ['t', 'stores', 'sync', orgSlug] })
      toast.success(t('stores.testConnected'))
    }
    window.addEventListener('storage', onStorage)
    return () => window.removeEventListener('storage', onStorage)
  }, [queryClient, orgSlug, t])

  const gdriveAuth = useMutation({
    mutationFn: (id: string) =>
      tenantApi<{ auth_url: string }>(`/api/t/stores/${id}/gdrive/auth-url`, orgSlug!, {
        method: 'POST',
        body: JSON.stringify({}),
      }),
    onMutate: () => setGdriveError(false),
  })

  const handleGdriveConnect = (storeId: string) => {
    setGdriveError(false)
    // Open a blank tab synchronously (inside the click gesture) so popup
    // blockers don't block it; navigate it once the auth URL arrives.
    const popup = window.open('', '_blank')
    gdriveAuth.mutate(storeId, {
      onSuccess: (data) => {
        if (popup && !popup.closed) {
          popup.location.href = data.auth_url
        } else {
          window.location.href = data.auth_url
        }
      },
      onError: () => {
        popup?.close()
        setGdriveError(true)
      },
    })
  }

  const storesQuery = useQuery({
    queryKey: ['t', 'stores', orgSlug],
    queryFn: () => tenantApi<StoresData>('/api/t/stores', orgSlug!),
    enabled: !!orgSlug,
  })

  const syncQuery = useQuery({
    queryKey: ['t', 'stores', 'sync', orgSlug],
    queryFn: () => tenantApi<SyncData>('/api/t/stores/sync', orgSlug!),
    enabled: !!orgSlug,
  })

  const invalidateStores = () => {
    queryClient.invalidateQueries({ queryKey: ['t', 'stores', orgSlug] })
    queryClient.invalidateQueries({ queryKey: ['t', 'stores', 'sync', orgSlug] })
  }

  const testStore = useMutation({
    mutationFn: (id: string) => tenantApi<TestStoreData>(`/api/t/stores/${id}/test`, orgSlug!, { method: 'POST' }),
    onSuccess: () => {
      toast.success(t('stores.testConnected'))
    },
    onError: () => {
      toast.error(t('stores.testFailed'))
    },
  })

  const setPrimary = useMutation({
    mutationFn: (id: string) => tenantApi<unknown>(`/api/t/stores/${id}/primary`, orgSlug!, { method: 'POST' }),
    onSuccess: invalidateStores,
  })

  const triggerIngest = useMutation({
    mutationFn: (id: string) => tenantApi<IngestData>(`/api/t/stores/${id}/ingest`, orgSlug!, { method: 'POST' }),
    onSuccess: invalidateStores,
  })

  const triggerSync = useMutation({
    mutationFn: () => tenantApi<TriggerSyncData>('/api/t/stores/sync', orgSlug!, { method: 'POST' }),
    onSuccess: invalidateStores,
  })

  const setStorageMode = useMutation({
    mutationFn: (mode: string) =>
      tenantApi<{ storageMode: string }>('/api/t/storage-mode', orgSlug!, {
        method: 'PATCH',
        body: JSON.stringify({ storage_mode: mode }),
      }),
    onSuccess: invalidateStores,
  })

  const stores = storesQuery.data?.stores ?? []
  const primaryStoreId = storesQuery.data?.primaryStoreId ?? null
  const storageMode = storesQuery.data?.storageMode ?? 'cumulative'
  const gdriveRedirectUri = storesQuery.data?.gdriveRedirectUri
  const runs = syncQuery.data?.runs ?? []

  // openEdit loads a store into the form so the shared dialog can update it.
  const openEdit = (store: Store) => {
    setForm({
      name: store.name,
      provider: store.provider as Provider,
      writeMode: store.write_mode as WriteMode,
      ingestMode: store.ingest_mode as IngestMode,
      readPriority: store.read_priority,
      quotaLimit: Math.round(bytesToGB(store.quota_limit)),
      config: { ...(store.config as Record<string, string>) },
      credentials: {},
    })
    setEditTarget(store)
    setCreateOpen(true)
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t('stores.title')}</h1>
          <p className="text-sm text-muted-foreground">{t('stores.description')}</p>
        </div>
        <Button
          onClick={() => {
            setEditTarget(null)
            setCreateOpen(true)
          }}
        >
          <Plus className="h-4 w-4 mr-2" />
          {t('stores.attachStore')}
        </Button>
      </div>

      {storesQuery.isPending && <p className="text-sm text-muted-foreground">...</p>}
      {storesQuery.isError && <p className="text-sm text-destructive">{t('stores.loadError')}</p>}
      {gdriveError && (
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
          {t('stores.gdriveConnectError')}
        </div>
      )}

      {stores.length === 0 && !storesQuery.isPending && (
        <Card>
          <CardContent className="py-10 text-center">
            <p className="text-sm text-muted-foreground">{t('stores.noStores')}</p>
            <p className="text-xs text-muted-foreground mt-1">{t('stores.noStoresHint')}</p>
          </CardContent>
        </Card>
      )}

      <StoreGroups
        stores={stores}
        primaryStoreId={primaryStoreId}
        onEdit={openEdit}
        onGdriveConnect={handleGdriveConnect}
        gdriveAuthPending={gdriveAuth.isPending}
        onSetPrimary={(id) => setPrimary.mutate(id)}
        setPrimaryPending={setPrimary.isPending}
        onTest={(id) => testStore.mutate(id)}
        testPending={testStore.isPending}
        onIngest={(id) => triggerIngest.mutate(id)}
        ingestPending={triggerIngest.isPending}
        onDelete={setDeleteTarget}
      />

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-base">
            <Database className="h-4 w-4" />
            {t('stores.storageMode')}
          </CardTitle>
          <CardDescription>{t('stores.storageModeHint')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3 text-sm">
          <Select value={storageMode} onValueChange={(v) => v && setStorageMode.mutate(v)}>
            <SelectTrigger className="max-w-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent className="min-w-64">
              <SelectItem value="cumulative">{t('stores.storageModeCumulative')}</SelectItem>
              <SelectItem value="replicate">{t('stores.storageModeReplicate')}</SelectItem>
            </SelectContent>
          </Select>
          <p className="text-xs text-muted-foreground">
            {storageMode === 'cumulative'
              ? t('stores.storageModeCumulativeDesc')
              : t('stores.storageModeReplicateDesc')}
          </p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-base">
            <RefreshCw className="h-4 w-4" />
            {t('stores.syncTitle')}
          </CardTitle>
          <CardDescription>
            {storageMode === 'replicate' ? (
              <Button variant="outline" size="sm" disabled={triggerSync.isPending} onClick={() => triggerSync.mutate()}>
                {t('stores.triggerSync')}
              </Button>
            ) : (
              <p className="text-xs text-muted-foreground">{t('stores.syncDisabledCumulative')}</p>
            )}
          </CardDescription>
        </CardHeader>
        {storageMode === 'replicate' && (
          <CardContent className="space-y-2 text-sm">
            {runs.length === 0 && <p className="text-muted-foreground">{t('stores.noRuns')}</p>}
            {runs.map((run) => (
              <div key={run.id} className="flex items-center justify-between rounded-lg border p-3">
                <div>
                  <p className="text-sm font-medium capitalize">{run.kind}</p>
                  <p className="text-xs text-muted-foreground">
                    {t('stores.runItems', { processed: run.processed_items, total: run.total_items })}
                    {run.failed_items > 0 && ` · ${t('stores.runFailed', { failed: run.failed_items })}`}
                  </p>
                </div>
                <Badge variant={run.status === 'completed' ? 'default' : 'secondary'} className="capitalize">
                  {run.status}
                </Badge>
              </div>
            ))}
          </CardContent>
        )}
      </Card>

      <Separator />

      <S3KeysCard orgSlug={orgSlug} s3Endpoint={s3Endpoint} isDev={isDev} />

      <StoreFormDialog
        orgSlug={orgSlug}
        createOpen={createOpen}
        setCreateOpen={setCreateOpen}
        editTarget={editTarget}
        setEditTarget={setEditTarget}
        deleteTarget={deleteTarget}
        setDeleteTarget={setDeleteTarget}
        form={form}
        setForm={setForm}
        showLocalHelp={showLocalHelp}
        setShowLocalHelp={setShowLocalHelp}
        showGdriveHelp={showGdriveHelp}
        setShowGdriveHelp={setShowGdriveHelp}
        gdriveRedirectUri={gdriveRedirectUri}
      />
    </div>
  )
}
