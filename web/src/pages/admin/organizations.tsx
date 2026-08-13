import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { adminApi } from '@/lib/api'
import type { AdminOrg, AdminOrgMember } from '@/lib/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/components/ui/dialog'
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
  AlertDialogAction,
} from '@/components/ui/alert-dialog'
import { Building2, Users, HardDrive } from 'lucide-react'
import { formatBytes, providerLabel } from '@/pages/app/stores/stores'

interface AdminOrgListData {
  orgs: AdminOrg[]
}

interface AdminOrgDetailData {
  org: AdminOrg
  members: AdminOrgMember[]
}

interface AdminOrgStorageData {
  organization: AdminOrg
  quota_limit: number
  stores: {
    id: string
    name: string
    provider: string
    status: string
    quota_limit: number
  }[]
}

const formatDate = (iso?: string) => {
  if (!iso) return '—'
  return new Date(iso).toLocaleDateString('id-ID', { day: '2-digit', month: 'short', year: 'numeric' })
}

export default function OrganizationsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const [renameTarget, setRenameTarget] = useState<AdminOrg | null>(null)
  const [renameName, setRenameName] = useState('')
  const [membersTarget, setMembersTarget] = useState<AdminOrg | null>(null)
  const [storageTarget, setStorageTarget] = useState<AdminOrg | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<AdminOrg | null>(null)

  const { data: orgs = [], isLoading, isError } = useQuery({
    queryKey: ['admin', 'orgs'],
    queryFn: () => adminApi<AdminOrgListData>('/api/admin/orgs'),
    select: (data) => data.orgs ?? [],
  })

  const { data: orgDetail, isLoading: membersLoading } = useQuery({
    queryKey: ['admin', 'orgs', membersTarget?.slug],
    queryFn: () => adminApi<AdminOrgDetailData>(`/api/admin/orgs/${membersTarget?.slug}`),
    enabled: !!membersTarget,
  })

  const { data: orgStorage, isLoading: storageLoading } = useQuery({
    queryKey: ['admin', 'orgs', storageTarget?.slug, 'storage'],
    queryFn: () => adminApi<AdminOrgStorageData>(`/api/admin/orgs/${storageTarget?.slug}/storage`),
    enabled: !!storageTarget,
  })

  const renameOrg = useMutation({
    mutationFn: (slug: string) =>
      adminApi<void>(`/api/admin/orgs/${slug}`, {
        method: 'PATCH',
        body: JSON.stringify({ name: renameName }),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'orgs'] })
      setRenameTarget(null)
    },
  })

  const deleteOrg = useMutation({
    mutationFn: (slug: string) => adminApi<void>(`/api/admin/orgs/${slug}`, { method: 'DELETE' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'orgs'] })
      setDeleteTarget(null)
    },
  })

  const filtered = orgs.filter(
    (o) =>
      o.name.toLowerCase().includes(search.toLowerCase()) ||
      o.slug.toLowerCase().includes(search.toLowerCase()),
  )

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <div className="rounded-full bg-primary/10 p-2">
          <Building2 className="h-5 w-5 text-primary" />
        </div>
        <h1 className="text-2xl font-bold">{t('admin.orgManagement')}</h1>
      </div>

      <div className="flex gap-2">
        <Input
          type="text"
          placeholder={t('admin.searchOrgs')}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="flex-1"
        />
      </div>

      {isLoading ? (
        <p className="text-muted-foreground">{t('app.loading')}</p>
      ) : isError ? (
        <p className="text-destructive">{t('admin.loadFailed')}</p>
      ) : (
        <div className="rounded-lg border border-border overflow-x-auto">
          <table className="w-full min-w-[640px] text-sm">
            <thead className="bg-muted/50">
              <tr>
                <th className="px-4 py-2 text-left">{t('admin.name')}</th>
                <th className="px-4 py-2 text-left">{t('admin.slug')}</th>
                <th className="px-4 py-2 text-center">{t('admin.members')}</th>
                <th className="px-4 py-2 text-left">{t('admin.attachedStorage')}</th>
                <th className="px-4 py-2 text-left">{t('admin.createdAt')}</th>
                <th className="px-4 py-2 text-right">{t('admin.actions')}</th>
              </tr>
            </thead>
            <tbody>
              {filtered.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-4 py-4 text-center text-muted-foreground">
                    {t('admin.noOrgs')}
                  </td>
                </tr>
              ) : (
                filtered.map((o) => (
                  <tr key={o.id} className="border-t hover:bg-muted/20">
                    <td className="px-4 py-3 font-medium">{o.name}</td>
                    <td className="px-4 py-3 text-muted-foreground">{o.slug}</td>
                    <td className="px-4 py-3 text-center">
                      <Badge variant="secondary">{o.member_count ?? 0}</Badge>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2 text-xs text-muted-foreground">
                        <HardDrive className="h-3.5 w-3.5" />
                        <span>
                          {o.store_count ?? 0}
                          {(o.gdrive_store_count ?? 0) > 0 && ` (${o.gdrive_store_count} gdrive)`}
                        </span>
                        {(o.store_capacity ?? 0) > 0 && (
                          <span>· {formatBytes(o.store_capacity ?? 0)}</span>
                        )}
                        {(o.attached_quota ?? 0) > 0 && (
                          <span>· {t('admin.quotaLimit')} {formatBytes(o.attached_quota ?? 0)}</span>
                        )}
                      </div>
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">{formatDate(o.created_at)}</td>
                    <td className="px-4 py-3 text-right space-x-1">
                      <Button variant="ghost" size="sm" onClick={() => setStorageTarget(o)}>
                        <HardDrive className="h-3.5 w-3.5 mr-1" />
                        {t('admin.viewStorage')}
                      </Button>
                      <Button variant="ghost" size="sm" onClick={() => setMembersTarget(o)}>
                        <Users className="h-3.5 w-3.5 mr-1" />
                        {t('admin.viewMembers')}
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => {
                          setRenameTarget(o)
                          setRenameName(o.name)
                        }}
                      >
                        {t('admin.rename')}
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-destructive"
                        onClick={() => setDeleteTarget(o)}
                      >
                        {t('admin.delete')}
                      </Button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      )}

      <Dialog open={!!renameTarget} onOpenChange={(open) => !open && setRenameTarget(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t('admin.renameOrg')}</DialogTitle>
            <DialogDescription>{renameTarget?.slug}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <Input
              value={renameName}
              onChange={(e) => setRenameName(e.target.value)}
              placeholder={t('admin.orgNamePlaceholder')}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRenameTarget(null)}>
              {t('admin.cancel')}
            </Button>
            <Button
              onClick={() => renameTarget && renameOrg.mutate(renameTarget.slug)}
              disabled={renameOrg.isPending || !renameName.trim()}
            >
              {renameOrg.isPending ? t('admin.saving') : t('admin.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!membersTarget} onOpenChange={(open) => !open && setMembersTarget(null)}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>
              {t('admin.membersOf', { name: membersTarget?.name ?? '' })}
            </DialogTitle>
          </DialogHeader>
          {membersLoading ? (
            <p className="text-muted-foreground">{t('app.loading')}</p>
          ) : !orgDetail || orgDetail.members.length === 0 ? (
            <p className="text-muted-foreground">{t('admin.noMembers')}</p>
          ) : (
            <div className="max-h-80 overflow-y-auto">
              <table className="w-full text-sm">
                <thead className="bg-muted/50">
                  <tr>
                    <th className="px-4 py-2 text-left">{t('admin.name')}</th>
                    <th className="px-4 py-2 text-left">{t('admin.email')}</th>
                    <th className="px-4 py-2 text-left">{t('admin.role')}</th>
                  </tr>
                </thead>
                <tbody>
                  {orgDetail.members.map((m) => (
                    <tr key={m.id} className="border-t">
                      <td className="px-4 py-2 font-medium">{m.name}</td>
                      <td className="px-4 py-2 text-muted-foreground">{m.email}</td>
                      <td className="px-4 py-2">
                        <Badge variant={m.role === 'owner' ? 'default' : 'secondary'}>{m.role}</Badge>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </DialogContent>
      </Dialog>

      <Dialog open={!!storageTarget} onOpenChange={(open) => !open && setStorageTarget(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t('admin.attachedStorage')}</DialogTitle>
            <DialogDescription>{storageTarget?.slug}</DialogDescription>
          </DialogHeader>
          {storageLoading ? (
            <p className="text-muted-foreground">{t('app.loading')}</p>
          ) : !orgStorage || orgStorage.stores.length === 0 ? (
            <p className="text-muted-foreground">{t('admin.noStores')}</p>
          ) : (
            <div className="max-h-80 overflow-y-auto">
              <table className="w-full text-sm">
                <thead className="bg-muted/50">
                  <tr>
                    <th className="px-4 py-2 text-left">{t('admin.name')}</th>
                    <th className="px-4 py-2 text-left">{t('admin.provider')}</th>
                    <th className="px-4 py-2 text-left">{t('admin.status')}</th>
                    <th className="px-4 py-2 text-right">{t('admin.quotaLimit')}</th>
                  </tr>
                </thead>
                <tbody>
                  {orgStorage.stores.map((s) => (
                    <tr key={s.id} className="border-t">
                      <td className="px-4 py-2 font-medium">{s.name}</td>
                      <td className="px-4 py-2 text-muted-foreground">{providerLabel(s.provider)}</td>
                      <td className="px-4 py-2">
                        <Badge variant={s.status === 'active' ? 'secondary' : 'outline'}>{s.status}</Badge>
                      </td>
                      <td className="px-4 py-2 text-right text-muted-foreground">
                        {s.quota_limit > 0 ? formatBytes(s.quota_limit) : t('admin.unlimited')}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {orgStorage && orgStorage.quota_limit > 0 && (
            <p className="text-sm text-muted-foreground">
              {t('admin.orgQuotaAllocation')}: {formatBytes(orgStorage.quota_limit)}
            </p>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => setStorageTarget(null)}>
              {t('admin.close')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('admin.deleteOrgTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('admin.deleteOrgConfirm')} <span className="font-medium">{deleteTarget?.name}</span>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('admin.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              disabled={deleteOrg.isPending}
              onClick={() => deleteTarget && deleteOrg.mutate(deleteTarget.slug)}
            >
              {deleteOrg.isPending ? t('admin.deleting') : t('admin.delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
