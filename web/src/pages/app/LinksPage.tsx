import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { tenantApi } from '@/lib/api'
import type { ShareLink, TrackedLink, TrackedLinkEvent, UploadLink } from '@/lib/types'
import { useOrgStore } from '@/store/org'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
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
import { Link2, Pencil, Plus, Trash2, Eye } from 'lucide-react'

interface LinksData<T> {
  links: T[]
}

interface EventsData {
  events: TrackedLinkEvent[]
}

type LinkKind = 'share' | 'upload' | 'tracked'

interface LinkForm {
  name: string
  access: string
  fileId: string
  folderId: string
  password: string
  expiresAt: string
  maxDownloads: string
  maxFiles: string
  maxFileSize: string
  requireEmail: boolean
  maxViews: string
  description: string
  isActive: boolean
}

const emptyForm: LinkForm = {
  name: '',
  access: 'download',
  fileId: '',
  folderId: '',
  password: '',
  expiresAt: '',
  maxDownloads: '',
  maxFiles: '',
  maxFileSize: '',
  requireEmail: false,
  maxViews: '',
  description: '',
  isActive: true,
}

const resourcePath = (kind: LinkKind) => {
  if (kind === 'share') return 'share-links'
  if (kind === 'upload') return 'upload-links'
  return 'tracked-links'
}

const renderCommon = (link: ShareLink | UploadLink | TrackedLink) => (
  <span className="font-mono text-xs text-muted-foreground truncate">{link.token}</span>
)

export default function LinksPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const currentOrg = useOrgStore((s) => s.currentOrg)
  const orgSlug = currentOrg?.slug

  const [activeTab, setActiveTab] = useState<LinkKind>('share')
  const [createOpen, setCreateOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<{ kind: LinkKind; id: string } | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<{ kind: LinkKind; id: string } | null>(null)
  const [form, setForm] = useState<LinkForm>(emptyForm)
  const [eventsFor, setEventsFor] = useState<string | null>(null)

  const shareQuery = useQuery({
    queryKey: ['t', 'share-links', orgSlug],
    queryFn: () => tenantApi<LinksData<ShareLink>>('/api/t/share-links', orgSlug!),
    enabled: !!orgSlug,
  })

  const uploadQuery = useQuery({
    queryKey: ['t', 'upload-links', orgSlug],
    queryFn: () => tenantApi<LinksData<UploadLink>>('/api/t/upload-links', orgSlug!),
    enabled: !!orgSlug,
  })

  const trackedQuery = useQuery({
    queryKey: ['t', 'tracked-links', orgSlug],
    queryFn: () => tenantApi<LinksData<TrackedLink>>('/api/t/tracked-links', orgSlug!),
    enabled: !!orgSlug,
  })

  const eventsQuery = useQuery({
    queryKey: ['t', 'tracked-links', eventsFor, 'events', orgSlug],
    queryFn: () => tenantApi<EventsData>(`/api/t/tracked-links/${eventsFor}/events`, orgSlug!),
    enabled: !!orgSlug && !!eventsFor,
  })

  const invalidateLinks = () => {
    queryClient.invalidateQueries({ queryKey: ['t', 'share-links', orgSlug] })
    queryClient.invalidateQueries({ queryKey: ['t', 'upload-links', orgSlug] })
    queryClient.invalidateQueries({ queryKey: ['t', 'tracked-links', orgSlug] })
  }

  const buildCreateBody = (kind: LinkKind) => {
    const base: Record<string, unknown> = {
      name: form.name,
      password: form.password,
      expiresAt: form.expiresAt ? new Date(form.expiresAt).toISOString() : null,
    }
    if (kind === 'share') {
      base.access = form.access || 'download'
      base.fileId = form.fileId || null
      base.folderId = form.folderId || null
      if (form.maxDownloads) base.maxDownloads = Number(form.maxDownloads)
    } else if (kind === 'upload') {
      base.folderId = form.folderId || null
      if (form.maxFiles) base.maxFiles = Number(form.maxFiles)
      if (form.maxFileSize) base.maxFileSize = Number(form.maxFileSize)
    } else {
      base.access = form.access || 'view'
      base.fileId = form.fileId || null
      base.folderId = form.folderId || null
      base.description = form.description
      base.requireEmail = form.requireEmail
      if (form.maxViews) base.maxViews = Number(form.maxViews)
    }
    return base
  }

  const buildUpdateBody = (kind: LinkKind) => {
    const body: Record<string, unknown> = { isActive: form.isActive }
    if (kind === 'share') {
      body.access = form.access || undefined
      if (form.password !== '') body.password = form.password
      if (form.expiresAt) body.expiresAt = new Date(form.expiresAt).toISOString()
      if (form.maxDownloads) body.maxDownloads = Number(form.maxDownloads)
    } else if (kind === 'upload') {
      body.name = form.name || undefined
      if (form.password !== '') body.password = form.password
      if (form.expiresAt) body.expiresAt = new Date(form.expiresAt).toISOString()
      if (form.maxFiles) body.maxFiles = Number(form.maxFiles)
      if (form.maxFileSize) body.maxFileSize = Number(form.maxFileSize)
    } else {
      body.name = form.name || undefined
      body.description = form.description || undefined
      body.access = form.access || undefined
      body.requireEmail = form.requireEmail
      if (form.password !== '') body.password = form.password
      if (form.expiresAt) body.expiresAt = new Date(form.expiresAt).toISOString()
      if (form.maxViews) body.maxViews = Number(form.maxViews)
    }
    return body
  }

  const createLink = useMutation({
    mutationFn: (kind: LinkKind) =>
      tenantApi<unknown>(`/api/t/${resourcePath(kind)}`, orgSlug!, {
        method: 'POST',
        body: JSON.stringify(buildCreateBody(kind)),
      }),
    onSuccess: () => {
      invalidateLinks()
      setCreateOpen(false)
      setForm(emptyForm)
    },
  })

  const updateLink = useMutation({
    mutationFn: ({ kind, id }: { kind: LinkKind; id: string }) =>
      tenantApi<unknown>(`/api/t/${resourcePath(kind)}/${id}`, orgSlug!, {
        method: 'PATCH',
        body: JSON.stringify(buildUpdateBody(kind)),
      }),
    onSuccess: () => {
      invalidateLinks()
      setEditTarget(null)
      setForm(emptyForm)
    },
  })

  const deleteLink = useMutation({
    mutationFn: ({ kind, id }: { kind: LinkKind; id: string }) =>
      tenantApi<unknown>(`/api/t/${resourcePath(kind)}/${id}`, orgSlug!, {
        method: 'DELETE',
      }),
    onSuccess: () => {
      invalidateLinks()
      setDeleteTarget(null)
    },
  })

  const shareLinks = shareQuery.data?.links ?? []
  const uploadLinks = uploadQuery.data?.links ?? []
  const trackedLinks = trackedQuery.data?.links ?? []
  const events = eventsQuery.data?.events ?? []

  const openCreate = (kind: LinkKind) => {
    setEditTarget(null)
    setForm({ ...emptyForm, access: kind === 'tracked' ? 'view' : 'download' })
    setCreateOpen(true)
  }

  const openEdit = (kind: LinkKind, link: ShareLink | UploadLink | TrackedLink) => {
    setCreateOpen(true)
    setEditTarget({ kind, id: link.id })
    setForm({
      name: 'name' in link ? link.name : '',
      access: 'access' in link ? link.access : kind === 'tracked' ? 'view' : 'download',
      fileId: 'file_id' in link && typeof link.file_id === 'string' ? link.file_id : '',
      folderId: link.folder_id ?? '',
      password: '',
      expiresAt: link.expires_at ? link.expires_at.slice(0, 10) : '',
      maxDownloads: 'max_downloads' in link && link.max_downloads ? String(link.max_downloads) : '',
      maxFiles: 'max_files' in link && link.max_files ? String(link.max_files) : '',
      maxFileSize: 'max_file_size' in link && link.max_file_size ? String(link.max_file_size) : '',
      requireEmail: 'require_email' in link ? link.require_email : false,
      maxViews: 'max_views' in link && link.max_views ? String(link.max_views) : '',
      description: 'description' in link ? link.description : '',
      isActive: link.is_active,
    })
  }

  const renderLinkActions = (
    kind: LinkKind,
    link: ShareLink | UploadLink | TrackedLink,
  ) => (
    <div className="flex items-center gap-1">
      {kind === 'tracked' && (
        <Button variant="ghost" size="icon" className="h-8 w-8" aria-label={t('links.events')} onClick={() => setEventsFor(link.id)}>
          <Eye className="h-4 w-4" />
        </Button>
      )}
      <Button variant="ghost" size="icon" className="h-8 w-8" aria-label={t('links.edit')} onClick={() => openEdit(kind, link)}>
        <Pencil className="h-4 w-4" />
      </Button>
      <Button variant="ghost" size="icon" className="h-8 w-8 text-destructive" aria-label={t('links.delete')} onClick={() => setDeleteTarget({ kind, id: link.id })}>
        <Trash2 className="h-4 w-4" />
      </Button>
    </div>
  )

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t('links.title')}</h1>
          <p className="text-sm text-muted-foreground">{t('links.description')}</p>
        </div>
        <Button onClick={() => openCreate(activeTab)}>
          <Plus className="h-4 w-4 mr-2" />
          {activeTab === 'share' ? t('links.createShare') : activeTab === 'upload' ? t('links.createUpload') : t('links.createTracked')}
        </Button>
      </div>

      <Tabs value={activeTab} onValueChange={(v) => setActiveTab(v as LinkKind)}>
        <TabsList>
          <TabsTrigger value="share">{t('links.tabShare')}</TabsTrigger>
          <TabsTrigger value="upload">{t('links.tabUpload')}</TabsTrigger>
          <TabsTrigger value="tracked">{t('links.tabTracked')}</TabsTrigger>
        </TabsList>

        <TabsContent value="share">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <Link2 className="h-4 w-4" />
                {t('links.tabShare')}
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              {shareLinks.length === 0 && <p className="text-sm text-muted-foreground">{t('links.noShareLinks')}</p>}
              {shareLinks.map((link) => (
                <div key={link.id} className="flex items-center gap-3 rounded-lg border p-3">
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium">{link.access} · {t('links.downloadCount', { count: link.download_count })}</p>
                    {renderCommon(link)}
                  </div>
                  <Badge variant={link.is_active ? 'default' : 'secondary'}>
                    {link.is_active ? t('links.active') : t('links.inactive')}
                  </Badge>
                  {renderLinkActions('share', link)}
                </div>
              ))}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="upload">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <Link2 className="h-4 w-4" />
                {t('links.tabUpload')}
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              {uploadLinks.length === 0 && <p className="text-sm text-muted-foreground">{t('links.noUploadLinks')}</p>}
              {uploadLinks.map((link) => (
                <div key={link.id} className="flex items-center gap-3 rounded-lg border p-3">
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium truncate">{link.name}</p>
                    {renderCommon(link)}
                  </div>
                  <Badge variant={link.is_active ? 'default' : 'secondary'}>
                    {link.is_active ? t('links.active') : t('links.inactive')}
                  </Badge>
                  {renderLinkActions('upload', link)}
                </div>
              ))}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="tracked">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <Link2 className="h-4 w-4" />
                {t('links.tabTracked')}
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              {trackedLinks.length === 0 && <p className="text-sm text-muted-foreground">{t('links.noTrackedLinks')}</p>}
              {trackedLinks.map((link) => (
                <div key={link.id} className="flex items-center gap-3 rounded-lg border p-3">
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium truncate">{link.name || link.access}</p>
                    <p className="text-xs text-muted-foreground">
                      {t('links.viewCount', { count: link.view_count })} · {t('links.downloadCount', { count: link.download_count })}
                    </p>
                    {renderCommon(link)}
                  </div>
                  <Badge variant={link.is_active ? 'default' : 'secondary'}>
                    {link.is_active ? t('links.active') : t('links.inactive')}
                  </Badge>
                  {renderLinkActions('tracked', link)}
                </div>
              ))}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {/* Create/Edit dialog */}
      <Dialog open={createOpen} onOpenChange={(o) => { if (!o) { setCreateOpen(false); setEditTarget(null); setForm(emptyForm) } }}>
        <DialogContent className="max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{editTarget ? t('links.edit') : t('links.create')}</DialogTitle>
            <DialogDescription>
              {editTarget ? t('links.edit') : t('links.create')} — {activeTab === 'share' ? t('links.tabShare') : activeTab === 'upload' ? t('links.tabUpload') : t('links.tabTracked')}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            {activeTab !== 'share' && (
              <div className="space-y-2">
                <Label htmlFor="l-name">{t('links.name')}</Label>
                <Input id="l-name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
              </div>
            )}
            {activeTab !== 'upload' && (
              <div className="space-y-2">
                <Label htmlFor="l-access">{t('links.access')}</Label>
                <Select value={form.access} onValueChange={(v) => setForm({ ...form, access: v ?? 'view' })}>
                  <SelectTrigger id="l-access">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="view">{t('links.accessView')}</SelectItem>
                    <SelectItem value="download">{t('links.accessDownload')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            )}
            {activeTab !== 'upload' && (
              <div className="space-y-2">
                <Label htmlFor="l-file">{t('links.fileId')}</Label>
                <Input id="l-file" value={form.fileId} onChange={(e) => setForm({ ...form, fileId: e.target.value })} placeholder="uuid" />
              </div>
            )}
            <div className="space-y-2">
              <Label htmlFor="l-folder">{t('links.folderId')}</Label>
              <Input id="l-folder" value={form.folderId} onChange={(e) => setForm({ ...form, folderId: e.target.value })} placeholder="uuid" />
            </div>
            {activeTab === 'tracked' && (
              <div className="space-y-2">
                <Label htmlFor="l-desc">{t('links.description')}</Label>
                <Input id="l-desc" value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} />
              </div>
            )}
            <div className="space-y-2">
              <Label htmlFor="l-password">{t('links.password')}</Label>
              <Input id="l-password" type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} placeholder={t('links.passwordHint')} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="l-expires">{t('links.expires')}</Label>
              <Input id="l-expires" type="date" value={form.expiresAt} onChange={(e) => setForm({ ...form, expiresAt: e.target.value })} />
            </div>
            {activeTab === 'share' && (
              <div className="space-y-2">
                <Label htmlFor="l-maxdl">{t('links.maxDownloads')}</Label>
                <Input id="l-maxdl" type="number" value={form.maxDownloads} onChange={(e) => setForm({ ...form, maxDownloads: e.target.value })} />
              </div>
            )}
            {activeTab === 'upload' && (
              <>
                <div className="space-y-2">
                  <Label htmlFor="l-maxfiles">{t('links.maxFiles')}</Label>
                  <Input id="l-maxfiles" type="number" value={form.maxFiles} onChange={(e) => setForm({ ...form, maxFiles: e.target.value })} />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="l-maxsize">{t('links.maxFileSize')}</Label>
                  <Input id="l-maxsize" type="number" value={form.maxFileSize} onChange={(e) => setForm({ ...form, maxFileSize: e.target.value })} />
                </div>
              </>
            )}
            {activeTab === 'tracked' && (
              <>
                <div className="space-y-2">
                  <Label htmlFor="l-maxviews">{t('links.maxViews')}</Label>
                  <Input id="l-maxviews" type="number" value={form.maxViews} onChange={(e) => setForm({ ...form, maxViews: e.target.value })} />
                </div>
                <div className="flex items-center gap-2">
                  <Switch id="l-email" checked={form.requireEmail} onCheckedChange={(v) => setForm({ ...form, requireEmail: v })} />
                  <Label htmlFor="l-email">{t('links.requireEmail')}</Label>
                </div>
              </>
            )}
            {editTarget && (
              <div className="flex items-center gap-2">
                <Switch id="l-active" checked={form.isActive} onCheckedChange={(v) => setForm({ ...form, isActive: v })} />
                <Label htmlFor="l-active">{t('links.status')}</Label>
              </div>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setCreateOpen(false); setEditTarget(null); setForm(emptyForm) }}>
              {t('links.cancel')}
            </Button>
            <Button
              disabled={(createLink.isPending || updateLink.isPending) || (activeTab !== 'share' && !form.name)}
              onClick={() => {
                const kind = activeTab
                if (editTarget) {
                  updateLink.mutate({ kind, id: editTarget.id })
                } else {
                  createLink.mutate(kind)
                }
              }}
            >
              {t('links.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete dialog */}
      <AlertDialog open={!!deleteTarget} onOpenChange={(o) => !o && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('links.delete')}</AlertDialogTitle>
            <AlertDialogDescription>{t('links.deleteConfirm')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => setDeleteTarget(null)}>{t('links.cancel')}</AlertDialogCancel>
            <AlertDialogAction variant="destructive" disabled={deleteLink.isPending} onClick={() => deleteTarget && deleteLink.mutate(deleteTarget)}>
              {t('links.delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Events dialog */}
      <Dialog open={!!eventsFor} onOpenChange={(o) => !o && setEventsFor(null)}>
        <DialogContent className="max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{t('links.events')}</DialogTitle>
            <DialogDescription>{t('links.tabTracked')}</DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            {events.length === 0 && <p className="text-sm text-muted-foreground">{t('links.noEvents')}</p>}
            {events.map((ev) => (
              <div key={ev.id} className="rounded-lg border p-3 text-sm">
                <div className="flex items-center justify-between">
                  <span className="font-medium capitalize">{ev.event_type}</span>
                  <span className="text-xs text-muted-foreground">{new Date(ev.timestamp).toLocaleString()}</span>
                </div>
                <p className="text-xs text-muted-foreground mt-1">
                  {[ev.browser, ev.os, ev.country].filter(Boolean).join(' · ') || '—'}
                </p>
              </div>
            ))}
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}