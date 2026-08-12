import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { tenantApi } from '@/lib/api'
import type { ShareLink, TrackedLink, UploadLink } from '@/lib/types'
import { useOrgStore } from '@/store/org'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Plus } from 'lucide-react'
import { DeleteLinkDialog } from './links/DeleteLinkDialog'
import { EventsDialog } from './links/EventsDialog'
import { LinkFormDialog } from './links/LinkFormDialog'
import { LinkListCard } from './links/LinkListCard'
import { LinkRow } from './links/LinkRow'
import { LinkToken } from './links/LinkToken'
import type { AnyLink, EventsData, LinkForm, LinkKind, LinksData } from './links/links'
import { emptyForm, resourcePath } from './links/links'

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

  const openEdit = (kind: LinkKind, link: AnyLink) => {
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

  const closeCreateDialog = () => {
    setCreateOpen(false)
    setEditTarget(null)
    setForm(emptyForm)
  }

  const handleSave = () => {
    const kind = activeTab
    if (editTarget) {
      updateLink.mutate({ kind, id: editTarget.id })
    } else {
      createLink.mutate(kind)
    }
  }

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

      <p className="text-sm text-muted-foreground">
        {activeTab === 'share' ? t('links.tabShareHelp') : activeTab === 'upload' ? t('links.tabUploadHelp') : t('links.tabTrackedHelp')}
      </p>

      <Tabs value={activeTab} onValueChange={(v) => setActiveTab(v as LinkKind)}>
        <TabsList>
          <TabsTrigger value="share">{t('links.tabShare')}</TabsTrigger>
          <TabsTrigger value="upload">{t('links.tabUpload')}</TabsTrigger>
          <TabsTrigger value="tracked">{t('links.tabTracked')}</TabsTrigger>
        </TabsList>

        <TabsContent value="share">
          <LinkListCard title={t('links.tabShare')} emptyLabel={t('links.noShareLinks')} isEmpty={shareLinks.length === 0}>
            {shareLinks.map((link) => (
              <LinkRow key={link.id} link={link} onEdit={() => openEdit('share', link)} onDelete={() => setDeleteTarget({ kind: 'share', id: link.id })}>
                <p className="text-sm font-medium">{link.access} · {t('links.downloadCount', { count: link.download_count })}</p>
                <LinkToken token={link.token} />
              </LinkRow>
            ))}
          </LinkListCard>
        </TabsContent>

        <TabsContent value="upload">
          <LinkListCard title={t('links.tabUpload')} emptyLabel={t('links.noUploadLinks')} isEmpty={uploadLinks.length === 0}>
            {uploadLinks.map((link) => (
              <LinkRow key={link.id} link={link} onEdit={() => openEdit('upload', link)} onDelete={() => setDeleteTarget({ kind: 'upload', id: link.id })}>
                <p className="text-sm font-medium truncate">{link.name}</p>
                <LinkToken token={link.token} />
              </LinkRow>
            ))}
          </LinkListCard>
        </TabsContent>

        <TabsContent value="tracked">
          <LinkListCard title={t('links.tabTracked')} emptyLabel={t('links.noTrackedLinks')} isEmpty={trackedLinks.length === 0}>
            {trackedLinks.map((link) => (
              <LinkRow key={link.id} link={link} onEdit={() => openEdit('tracked', link)} onDelete={() => setDeleteTarget({ kind: 'tracked', id: link.id })} onShowEvents={() => setEventsFor(link.id)}>
                <p className="text-sm font-medium truncate">{link.name || link.access}</p>
                <p className="text-xs text-muted-foreground">
                  {t('links.viewCount', { count: link.view_count })} · {t('links.downloadCount', { count: link.download_count })}
                </p>
                <LinkToken token={link.token} />
              </LinkRow>
            ))}
          </LinkListCard>
        </TabsContent>
      </Tabs>

      <LinkFormDialog
        open={createOpen}
        kind={activeTab}
        editTarget={editTarget}
        form={form}
        onFormChange={setForm}
        isPending={createLink.isPending || updateLink.isPending}
        onSave={handleSave}
        onCancel={closeCreateDialog}
      />
      <DeleteLinkDialog
        target={deleteTarget}
        isPending={deleteLink.isPending}
        onConfirm={() => deleteTarget && deleteLink.mutate(deleteTarget)}
        onClose={() => setDeleteTarget(null)}
      />
      <EventsDialog eventsFor={eventsFor} events={events} onClose={() => setEventsFor(null)} />
    </div>
  )
}
