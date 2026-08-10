import { useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router'
import { tenantApi } from '@/lib/api'
import type { ReplicationRun, S3Key, Store } from '@/lib/types'
import { useOrgStore } from '@/store/org'
import { Button, buttonVariants } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Progress } from '@/components/ui/progress'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
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
import { Check, Database, ExternalLink, HelpCircle, KeyRound, Loader2, Plus, RefreshCw, Star, Trash2 } from 'lucide-react'
import { toast } from 'sonner'

// formatBytes renders a byte count in a human-readable unit (KB/MB/GB/TB).
function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)))
  const value = bytes / 1024 ** i
  return `${value.toFixed(value >= 10 || i === 0 ? 0 : 1)} ${units[i]}`
}

function copyText(text: string) {
  void navigator.clipboard.writeText(text)
}

// localStorage key used to notify other tabs when a Google Drive connect
// completes in the OAuth tab (the `storage` event fires in every other tab).
const GDRIVE_CONNECTED_KEY = 'gdrive:connected'

interface StoresData {
  stores: Store[]
  primaryStoreId: string | null
  storageMode?: string
  gdriveRedirectUri?: string
}

interface TestStoreData {
  ok: boolean
  used: number
  limit: number
}

interface IngestData {
  ingested: number
}

interface SyncData {
  runs: ReplicationRun[]
}

interface TriggerSyncData {
  run: ReplicationRun
}

interface KeysData {
  keys: S3Key[]
}

interface CreateKeyData {
  key: S3Key
  accessKeyId: string
  secretAccessKey: string
}

type Provider = 'local' | 's3' | 'gdrive'

interface StoreForm {
  name: string
  provider: Provider
  writeMode: string
  ingestMode: string
  readPriority: number
  config: Record<string, string>
  credentials: Record<string, string>
}

interface GDriveCompleteData {
  ok: boolean
  used: number
  limit: number
  storeId: string
}

function useOrgSlug(): string | undefined {
  return useOrgStore((s) => s.currentOrg?.slug)
}

export default function StoresPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const orgSlug = useOrgSlug()

  const [createOpen, setCreateOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<Store | null>(null)
  const [keyCreatedData, setKeyCreatedData] = useState<CreateKeyData | null>(null)
  const [deleteKeyTarget, setDeleteKeyTarget] = useState<S3Key | null>(null)
  const [createKeyOpen, setCreateKeyOpen] = useState(false)
  const [showGdriveHelp, setShowGdriveHelp] = useState(false)
  const [showLocalHelp, setShowLocalHelp] = useState(false)

  // Create store form state
  const [form, setForm] = useState<StoreForm>({
    name: '',
    provider: 'local',
    writeMode: 'write',
    ingestMode: 'none',
    readPriority: 100,
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

  // S3 key form state
  const [keyName, setKeyName] = useState('')
  const [keyPermissions, setKeyPermissions] = useState('readwrite')

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

  const keysQuery = useQuery({
    queryKey: ['t', 's3keys', orgSlug],
    queryFn: () => tenantApi<KeysData>('/api/t/s3-keys', orgSlug!),
    enabled: !!orgSlug,
  })

  const invalidateStores = () => {
    queryClient.invalidateQueries({ queryKey: ['t', 'stores', orgSlug] })
    queryClient.invalidateQueries({ queryKey: ['t', 'stores', 'sync', orgSlug] })
  }

  const createStore = useMutation({
    mutationFn: (f: StoreForm) =>
      tenantApi<{ store: Store }>('/api/t/stores', orgSlug!, {
        method: 'POST',
        body: JSON.stringify({
          name: f.name,
          provider: f.provider,
          writeMode: f.writeMode,
          ingestMode: f.ingestMode,
          readPriority: f.readPriority,
          config: f.config,
          credentials: f.credentials,
        }),
      }),
    onSuccess: () => {
      invalidateStores()
      setCreateOpen(false)
      setForm({ name: '', provider: 'local', writeMode: 'write', ingestMode: 'none', readPriority: 100, config: {}, credentials: {} })
    },
  })

  const deleteStore = useMutation({
    mutationFn: (id: string) => tenantApi<unknown>(`/api/t/stores/${id}`, orgSlug!, { method: 'DELETE' }),
    onSuccess: () => {
      invalidateStores()
      setDeleteTarget(null)
    },
  })

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

  const createKey = useMutation({
    mutationFn: () =>
      tenantApi<CreateKeyData>('/api/t/s3-keys', orgSlug!, {
        method: 'POST',
        body: JSON.stringify({ name: keyName, permissions: keyPermissions }),
      }),
    onSuccess: (data) => {
      setKeyCreatedData(data)
      setKeyName('')
      setKeyPermissions('readwrite')
      queryClient.invalidateQueries({ queryKey: ['t', 's3keys', orgSlug] })
    },
  })

  const deleteKey = useMutation({
    mutationFn: (id: string) => tenantApi<unknown>(`/api/t/s3-keys/${id}`, orgSlug!, { method: 'DELETE' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['t', 's3keys', orgSlug] })
      setDeleteKeyTarget(null)
    },
  })

  const stores = storesQuery.data?.stores ?? []
  const primaryStoreId = storesQuery.data?.primaryStoreId ?? null
  const storageMode = storesQuery.data?.storageMode ?? 'cumulative'
  const gdriveRedirectUri = storesQuery.data?.gdriveRedirectUri
  const runs = syncQuery.data?.runs ?? []
  const keys = keysQuery.data?.keys ?? []

  const setConfigField = (key: string, value: string) => {
    setForm((f) => ({ ...f, config: { ...f.config, [key]: value } }))
  }

  const setCredentialField = (key: string, value: string) => {
    setForm((f) => ({ ...f, credentials: { ...f.credentials, [key]: value } }))
  }

  const providerLabel = (p: string) => {
    if (p === 'local') return t('stores.providerLocal')
    if (p === 's3') return t('stores.providerS3')
    if (p === 'gdrive') return t('stores.providerGdrive')
    return p
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t('stores.title')}</h1>
          <p className="text-sm text-muted-foreground">{t('stores.description')}</p>
        </div>
        <Button onClick={() => setCreateOpen(true)}>
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

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        {stores.map((store) => (
          <Card key={store.id}>
            <CardHeader className="pb-3">
              <CardTitle className="flex items-center gap-2 text-base">
                <Database className="h-4 w-4 text-muted-foreground" />
                <span className="truncate">{store.name}</span>
                {store.provider === 'gdrive' && store.status === 'active' && (
                  <Badge variant="outline" className="gap-1 border-emerald-600/30 bg-emerald-600/10 text-emerald-700">
                    <Check className="h-3 w-3" />
                    {t('stores.connected')}
                  </Badge>
                )}
                {store.provider === 'gdrive' && store.status === 'pending' && (
                  <Badge variant="outline" className="gap-1 border-amber-500/30 bg-amber-500/10 text-amber-700">
                    {t('stores.pending')}
                  </Badge>
                )}
                {store.provider === 'gdrive' && store.status === 'error' && (
                  <Badge variant="destructive" className="gap-1">
                    {t('stores.error')}
                  </Badge>
                )}
                {store.id === primaryStoreId && (
                  <Badge className="gap-1">
                    <Star className="h-3 w-3" />
                    {t('stores.primary')}
                  </Badge>
                )}
              </CardTitle>
              <CardDescription>
                <Badge variant="secondary">{providerLabel(store.provider)}</Badge>
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3 text-sm">
              <dl className="grid grid-cols-2 gap-2 text-xs">
                <div>
                  <dt className="text-muted-foreground">{t('stores.status')}</dt>
                  <dd className="capitalize">{store.status}</dd>
                </div>
                <div>
                  <dt className="text-muted-foreground">{t('stores.writeMode')}</dt>
                  <dd>{store.write_mode}</dd>
                </div>
                <div>
                  <dt className="text-muted-foreground">{t('stores.ingestMode')}</dt>
                  <dd>{store.ingest_mode}</dd>
                </div>
                <div>
                  <dt className="text-muted-foreground">{t('stores.readPriority')}</dt>
                  <dd>{store.read_priority}</dd>
                </div>
              </dl>

              {store.provider === 'gdrive' && store.status === 'active' && (
                <div className="space-y-1">
                  <Progress
                    value={store.quota_limit > 0 ? Math.min(100, (store.quota_used / store.quota_limit) * 100) : 0}
                    className="h-2"
                  />
                  {store.quota_limit > 0 && (
                    <p className="text-xs text-muted-foreground">
                      {t('stores.quotaUsage', {
                        used: formatBytes(store.quota_used),
                        limit: formatBytes(store.quota_limit),
                      })}
                    </p>
                  )}
                </div>
              )}

              <div className="flex flex-wrap gap-2">
                {store.provider === 'gdrive' && store.status !== 'active' && (
                  <Button variant="outline" size="sm" disabled={gdriveAuth.isPending} onClick={() => handleGdriveConnect(store.id)}>
                    {gdriveAuth.isPending ? <Loader2 className="h-3 w-3 animate-spin" /> : <RefreshCw className="h-3 w-3 mr-1" />}
                    {t('stores.connectGoogle')}
                  </Button>
                )}
                {store.id !== primaryStoreId && (
                  <Button variant="outline" size="sm" disabled={setPrimary.isPending} onClick={() => setPrimary.mutate(store.id)}>
                    <Star className="h-3 w-3 mr-1" />
                    {t('stores.setPrimary')}
                  </Button>
                )}
                <Button variant="outline" size="sm" disabled={testStore.isPending} onClick={() => testStore.mutate(store.id)}>
                  {testStore.isPending ? <Loader2 className="h-3 w-3 animate-spin" /> : <Check className="h-3 w-3 mr-1" />}
                  {t('stores.testConnection')}
                </Button>
                <Button variant="outline" size="sm" disabled={triggerIngest.isPending} onClick={() => triggerIngest.mutate(store.id)}>
                  {t('stores.triggerIngest')}
                </Button>
                <Button variant="ghost" size="sm" className="text-destructive" onClick={() => setDeleteTarget(store)}>
                  <Trash2 className="h-3 w-3 mr-1" />
                  {t('stores.deleteStore')}
                </Button>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

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
            <SelectContent>
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

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-base">
            <KeyRound className="h-4 w-4" />
            {t('stores.s3Keys')}
          </CardTitle>
          <CardDescription>
            <Button variant="outline" size="sm" onClick={() => setCreateKeyOpen(true)}>
              {t('stores.createKey')}
            </Button>
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          {keys.length === 0 && <p className="text-muted-foreground">{t('stores.noKeys')}</p>}
          {keys.map((key) => (
            <div key={key.id} className="flex items-center justify-between rounded-lg border p-3">
              <div>
                <p className="text-sm font-medium">{key.name}</p>
                <p className="font-mono text-xs text-muted-foreground">{key.access_key_id}</p>
              </div>
              <div className="flex items-center gap-2">
                <Badge variant="secondary">{key.permissions}</Badge>
                <Button variant="ghost" size="sm" className="text-destructive" onClick={() => setDeleteKeyTarget(key)}>
                  <Trash2 className="h-3 w-3" />
                </Button>
              </div>
            </div>
          ))}
        </CardContent>
      </Card>

      {/* Attach store dialog */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{t('stores.createStore')}</DialogTitle>
            <DialogDescription>
              {t('stores.description')}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="store-name">{t('stores.name')}</Label>
                <Input id="store-name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
              </div>
              <div className="space-y-2">
                <Label htmlFor="store-provider">{t('stores.provider')}</Label>
                <Select value={form.provider} onValueChange={(v) => setForm({ ...form, provider: v as Provider })}>
                  <SelectTrigger id="store-provider">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="local">{t('stores.providerLocal')}</SelectItem>
                    <SelectItem value="s3">{t('stores.providerS3')}</SelectItem>
                    <SelectItem value="gdrive">{t('stores.providerGdrive')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="space-y-2">
              <Label>{t('stores.configSection')}</Label>
              {form.provider === 'local' && (
                <>
                  <Input placeholder={t('stores.baseDirPlaceholder')} onChange={(e) => setConfigField('baseDir', e.target.value)} />
                  <Input placeholder={t('stores.publicUrlPlaceholder')} onChange={(e) => setConfigField('publicUrl', e.target.value)} />
                  <Button type="button" variant="ghost" size="sm" className="h-7 px-2" onClick={() => setShowLocalHelp(true)}>
                    <HelpCircle className="h-3.5 w-3.5 mr-1" />
                    {t('stores.localHelpTrigger')}
                  </Button>
                </>
              )}
              {form.provider === 's3' && (
                <>
                  <Input placeholder={t('stores.bucketPlaceholder')} onChange={(e) => setConfigField('bucket', e.target.value)} />
                  <Input placeholder={t('stores.regionPlaceholder')} onChange={(e) => setConfigField('region', e.target.value)} />
                  <Input placeholder={t('stores.endpointPlaceholder')} onChange={(e) => setConfigField('endpoint', e.target.value)} />
                </>
              )}
              {form.provider === 'gdrive' && (
                <Input placeholder={t('stores.folderIdPlaceholder')} onChange={(e) => setConfigField('folderId', e.target.value)} />
              )}
            </div>

            <div className="space-y-2">
              <Label>{t('stores.credentialsSection')}</Label>
              {form.provider === 's3' && (
                <>
                  <Input placeholder={t('stores.accessKeyId')} onChange={(e) => setCredentialField('accessKeyId', e.target.value)} />
                  <Input type="password" placeholder={t('stores.secretAccessKey')} onChange={(e) => setCredentialField('secretAccessKey', e.target.value)} />
                </>
              )}
              {form.provider === 'gdrive' && (
                <>
                  <Input placeholder={t('stores.clientId')} onChange={(e) => setCredentialField('clientId', e.target.value)} />
                  <Input type="password" placeholder={t('stores.clientSecret')} onChange={(e) => setCredentialField('clientSecret', e.target.value)} />
                  <Button type="button" variant="ghost" size="sm" className="h-7 px-2" onClick={() => setShowGdriveHelp(true)}>
                    <HelpCircle className="h-3.5 w-3.5 mr-1" />
                    {t('stores.gdriveHelpTrigger')}
                  </Button>
                </>
              )}
            </div>

            <div className="space-y-2">
              <Label>{t('stores.writeMode')}</Label>
              <Select value={form.writeMode} onValueChange={(v) => setForm({ ...form, writeMode: v })}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="write">write</SelectItem>
                  <SelectItem value="writeonly">writeonly</SelectItem>
                  <SelectItem value="none">none</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label>{t('stores.ingestMode')}</Label>
              <Select value={form.ingestMode} onValueChange={(v) => setForm({ ...form, ingestMode: v })}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">none</SelectItem>
                  <SelectItem value="poll">poll</SelectItem>
                  <SelectItem value="webhook">webhook</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label htmlFor="store-priority">{t('stores.readPriority')}</Label>
              <Input
                id="store-priority"
                type="number"
                value={form.readPriority}
                onChange={(e) => setForm({ ...form, readPriority: Number(e.target.value) })}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>
              {t('links.cancel')}
            </Button>
            <Button disabled={!form.name || createStore.isPending} onClick={() => createStore.mutate(form)}>
              {t('stores.createStore')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Local storage best practices dialog */}
      <Dialog open={showLocalHelp} onOpenChange={setShowLocalHelp}>
        <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{t('stores.localHelpTitle')}</DialogTitle>
            <DialogDescription>{t('stores.localHelpSubtitle')}</DialogDescription>
          </DialogHeader>
          <ul className="list-disc space-y-2.5 pl-5 text-sm marker:text-primary">
            <li>{t('stores.localHelpTip1')}</li>
            <li>{t('stores.localHelpTip2')}</li>
            <li>{t('stores.localHelpTip3')}</li>
            <li>{t('stores.localHelpTip4')}</li>
            <li>{t('stores.localHelpTip5')}</li>
          </ul>
        </DialogContent>
      </Dialog>

      {/* GDrive credentials help dialog */}
      <Dialog open={showGdriveHelp} onOpenChange={setShowGdriveHelp}>
        <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{t('stores.gdriveHelpTitle')}</DialogTitle>
            <DialogDescription>{t('stores.gdriveHelpSubtitle')}</DialogDescription>
          </DialogHeader>
          <ol className="list-decimal space-y-2.5 pl-5 text-sm marker:font-medium marker:text-primary">
            <li>{t('stores.gdriveHelpStep1')}</li>
            <li>{t('stores.gdriveHelpStep2')}</li>
            <li>{t('stores.gdriveHelpStep3')}</li>
            <li>{t('stores.gdriveHelpStep4')}</li>
            <li>{t('stores.gdriveHelpStep5')}</li>
            <li>
              {t('stores.gdriveHelpStep6')}
              {gdriveRedirectUri && (
                <code className="mt-2 block break-all rounded-md bg-muted px-2 py-1 text-xs">{gdriveRedirectUri}</code>
              )}
            </li>
            <li>{t('stores.gdriveHelpStep7')}</li>
          </ol>
          <div className="flex justify-end">
            <a
              href="https://console.cloud.google.com/"
              target="_blank"
              rel="noreferrer"
              className={buttonVariants({ variant: 'outline', size: 'sm' })}
            >
              <ExternalLink className="h-3.5 w-3.5 mr-1" />
              {t('stores.gdriveOpenConsole')}
            </a>
          </div>
        </DialogContent>
      </Dialog>

      {/* Delete store confirm */}
      <AlertDialog open={!!deleteTarget} onOpenChange={(o) => !o && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('stores.deleteStore')}</AlertDialogTitle>
            <AlertDialogDescription>
              {deleteTarget ? t('stores.deleteStoreConfirm', { name: deleteTarget.name }) : ''}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => setDeleteTarget(null)}>{t('links.cancel')}</AlertDialogCancel>
            <AlertDialogAction variant="destructive" disabled={deleteStore.isPending} onClick={() => deleteTarget && deleteStore.mutate(deleteTarget.id)}>
              {t('stores.deleteStore')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Create S3 API key dialog */}
      <Dialog open={createKeyOpen} onOpenChange={setCreateKeyOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('stores.createKeyTitle')}</DialogTitle>
            <DialogDescription>{t('stores.permissions')}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="key-name">{t('stores.keyName')}</Label>
              <Input
                id="key-name"
                value={keyName}
                onChange={(e) => setKeyName(e.target.value)}
                placeholder={t('stores.keyName')}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="key-permissions">{t('stores.permissions')}</Label>
              <Select value={keyPermissions} onValueChange={setKeyPermissions}>
                <SelectTrigger id="key-permissions">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="readwrite">{t('stores.permissionReadwrite')}</SelectItem>
                  <SelectItem value="readonly">{t('stores.permissionReadonly')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateKeyOpen(false)}>
              {t('links.cancel')}
            </Button>
            <Button
              disabled={!keyName.trim() || createKey.isPending}
              onClick={() => createKey.mutate()}
            >
              {createKey.isPending && <Loader2 className="h-4 w-4 animate-spin mr-2" />}
              {t('stores.createKey')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* S3 key created dialog */}
      <Dialog open={!!keyCreatedData} onOpenChange={(o) => !o && setKeyCreatedData(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('stores.keyCreated')}</DialogTitle>
            <DialogDescription>{t('stores.keyCreatedOnce')}</DialogDescription>
          </DialogHeader>
          {keyCreatedData && (
            <div className="space-y-3">
              <div className="space-y-2">
                <Label>{t('stores.accessKeyId')}</Label>
                <div className="flex gap-2">
                  <Input readOnly value={keyCreatedData.accessKeyId} />
                  <Button variant="outline" onClick={() => copyText(keyCreatedData.accessKeyId)}>
                    {t('stores.copyAccessKey')}
                  </Button>
                </div>
              </div>
              <div className="space-y-2">
                <Label>{t('stores.secretAccessKey')}</Label>
                <div className="flex gap-2">
                  <Input readOnly value={keyCreatedData.secretAccessKey} />
                  <Button variant="outline" onClick={() => copyText(keyCreatedData.secretAccessKey)}>
                    {t('stores.copySecret')}
                  </Button>
                </div>
              </div>
            </div>
          )}
          <DialogFooter>
            <Button onClick={() => setKeyCreatedData(null)}>{t('links.save')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete key confirm */}
      <AlertDialog open={!!deleteKeyTarget} onOpenChange={(o) => !o && setDeleteKeyTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('stores.deleteKey')}</AlertDialogTitle>
            <AlertDialogDescription>
              {deleteKeyTarget ? t('stores.deleteKeyConfirm', { name: deleteKeyTarget.name }) : ''}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => setDeleteKeyTarget(null)}>{t('links.cancel')}</AlertDialogCancel>
            <AlertDialogAction variant="destructive" disabled={deleteKey.isPending} onClick={() => deleteKeyTarget && deleteKey.mutate(deleteKeyTarget.id)}>
              {t('stores.deleteKey')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}