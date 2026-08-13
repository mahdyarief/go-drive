import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { adminApi } from '@/lib/api'
import type { AdminUser } from '@/lib/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
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
import { ShieldCheck, Shield, Users, Trash2, UserPlus, Pencil, HardDrive } from 'lucide-react'
import { formatBytes } from '@/pages/app/stores/stores'

interface AdminUserListData {
  users: AdminUser[]
}

const formatDate = (iso: string) => {
  return new Date(iso).toLocaleDateString('id-ID', { day: '2-digit', month: 'short', year: 'numeric' })
}

export default function UsersPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<AdminUser | null>(null)
  const [createOpen, setCreateOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<AdminUser | null>(null)
  const [quotaTarget, setQuotaTarget] = useState<AdminUser | null>(null)
  const [quotaLimit, setQuotaLimit] = useState('')

  const { data: users = [], isLoading, isError } = useQuery({
    queryKey: ['admin', 'users'],
    queryFn: () => adminApi<AdminUserListData>('/api/admin/users'),
    select: (data) => data.users ?? [],
  })

  const toggleAdmin = useMutation({
    mutationFn: (user: AdminUser) =>
      adminApi<void>(`/api/admin/users/${user.id}`, {
        method: 'PATCH',
        body: JSON.stringify({ is_admin: !user.is_admin }),
      }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'users'] }),
  })

  const deleteUser = useMutation({
    mutationFn: (id: string) => adminApi<void>(`/api/admin/users/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'users'] })
      setDeleteTarget(null)
    },
  })

  const createUser = useMutation({
    mutationFn: (data: { name: string; email: string; password: string; is_admin: boolean }) =>
      adminApi<void>('/api/admin/users', { method: 'POST', body: JSON.stringify(data) }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'users'] })
      setCreateOpen(false)
    },
  })

  const updateUser = useMutation({
    mutationFn: ({ id, data }: { id: string; data: { name: string; email: string; password?: string } }) =>
      adminApi<void>(`/api/admin/users/${id}`, { method: 'PATCH', body: JSON.stringify(data) }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'users'] })
      setEditTarget(null)
    },
  })

  const setUserQuota = useMutation({
    mutationFn: ({ id, limit }: { id: string; limit: number }) =>
      adminApi<void>(`/api/admin/users/${id}/limit`, {
        method: 'PATCH',
        body: JSON.stringify({ limit }),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'users'] })
      setQuotaTarget(null)
      setQuotaLimit('')
    },
  })

  const filtered = users.filter(
    (u) =>
      u.name.toLowerCase().includes(search.toLowerCase()) ||
      u.email.toLowerCase().includes(search.toLowerCase()),
  )

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <div className="rounded-full bg-primary/10 p-2">
          <Users className="h-5 w-5 text-primary" />
        </div>
        <h1 className="text-2xl font-bold">{t('admin.userManagement')}</h1>
      </div>

      <div className="flex gap-2">
        <Input
          type="text"
          placeholder={t('admin.searchUsers')}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="flex-1"
        />
        <Button onClick={() => setCreateOpen(true)}>
          <UserPlus className="h-4 w-4 mr-1.5" />
          {t('admin.addUser')}
        </Button>
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
                <th className="px-4 py-2 text-left">{t('admin.email')}</th>
                <th className="px-4 py-2 text-left">{t('admin.createdAt')}</th>
                <th className="px-4 py-2 text-center">{t('admin.orgs')}</th>
                <th className="px-4 py-2 text-left">{t('admin.quota')}</th>
                <th className="px-4 py-2 text-center">{t('admin.status')}</th>
                <th className="px-4 py-2 text-right">{t('admin.actions')}</th>
              </tr>
            </thead>
            <tbody>
              {filtered.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-4 py-4 text-center text-muted-foreground">
                    {t('admin.noUsers')}
                  </td>
                </tr>
              ) : (
                filtered.map((u) => (
                  <tr key={u.id} className="border-t hover:bg-muted/20">
                    <td className="px-4 py-3 font-medium">{u.name}</td>
                    <td className="px-4 py-3 text-muted-foreground">{u.email}</td>
                    <td className="px-4 py-3 text-muted-foreground">{formatDate(u.created_at)}</td>
                    <td className="px-4 py-3 text-center">
                      <Badge variant="secondary">{u.org_count}</Badge>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <span className="text-xs">
                          {u.quota_limit > 0 ? formatBytes(u.quota_limit) : t('admin.unlimited')}
                          {u.quota_allocated > 0 && (
                            <span className="text-muted-foreground">
                              {' '}
                              · {t('admin.allocated')} {formatBytes(u.quota_allocated)}
                            </span>
                          )}
                        </span>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-6 px-1.5"
                          onClick={() => {
                            setQuotaTarget(u)
                            setQuotaLimit(u.quota_limit > 0 ? String(Math.round(u.quota_limit / 1024 ** 3)) : '')
                          }}
                        >
                          <HardDrive className="h-3.5 w-3.5" />
                        </Button>
                      </div>
                    </td>
                    <td className="px-4 py-3 text-center">
                      {u.is_admin ? (
                        <Badge className="bg-emerald-500/10 text-emerald-600">
                          <ShieldCheck className="h-3 w-3 mr-1" />
                          {t('admin.admin')}
                        </Badge>
                      ) : (
                        <Badge variant="secondary">{t('admin.user')}</Badge>
                      )}
                    </td>
                    <td className="px-4 py-3 text-right space-x-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => toggleAdmin.mutate(u)}
                        disabled={toggleAdmin.isPending}
                      >
                        {u.is_admin ? (
                          <>
                            <Shield className="h-3.5 w-3.5 mr-1" />
                            {t('admin.revokeAdmin')}
                          </>
                        ) : (
                          <>
                            <ShieldCheck className="h-3.5 w-3.5 mr-1" />
                            {t('admin.grantAdmin')}
                          </>
                        )}
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setEditTarget(u)}
                      >
                        <Pencil className="h-3.5 w-3.5 mr-1" />
                        {t('admin.editUser')}
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-destructive"
                        onClick={() => setDeleteTarget(u)}
                      >
                        <Trash2 className="h-3.5 w-3.5 mr-1" />
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

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('admin.addUser')}</DialogTitle>
            <DialogDescription>{t('admin.userManagement')}</DialogDescription>
          </DialogHeader>
          <form
            className="space-y-4"
            onSubmit={(e) => {
              e.preventDefault()
              const form = new FormData(e.currentTarget)
              createUser.mutate({
                name: String(form.get('name') ?? ''),
                email: String(form.get('email') ?? ''),
                password: String(form.get('password') ?? ''),
                is_admin: form.get('is_admin') === 'on',
              })
            }}
          >
            <div className="space-y-2">
              <Label htmlFor="create-name">{t('admin.name')}</Label>
              <Input id="create-name" name="name" required />
            </div>
            <div className="space-y-2">
              <Label htmlFor="create-email">{t('admin.email')}</Label>
              <Input id="create-email" name="email" type="email" required />
            </div>
            <div className="space-y-2">
              <Label htmlFor="create-password">{t('admin.password')}</Label>
              <Input
                id="create-password"
                name="password"
                type="password"
                minLength={8}
                placeholder={t('form.passwordMinPlaceholder')}
                required
              />
            </div>
            <div className="flex items-center justify-between">
              <Label htmlFor="create-admin">{t('admin.admin')}</Label>
              <Switch id="create-admin" name="is_admin" />
            </div>
            {createUser.isError && (
              <p className="text-sm text-destructive">{createUser.error?.message}</p>
            )}
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setCreateOpen(false)}>
                {t('admin.cancel')}
              </Button>
              <Button type="submit" disabled={createUser.isPending}>
                {createUser.isPending ? t('admin.creating') : t('admin.save')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={!!editTarget} onOpenChange={(open) => !open && setEditTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('admin.editUser')}</DialogTitle>
            <DialogDescription>{editTarget?.email}</DialogDescription>
          </DialogHeader>
          <form
            key={editTarget?.id}
            className="space-y-4"
            onSubmit={(e) => {
              e.preventDefault()
              if (!editTarget) return
              const form = new FormData(e.currentTarget)
              const name = String(form.get('name') ?? '')
              const email = String(form.get('email') ?? '')
              const password = String(form.get('password') ?? '')
              updateUser.mutate({
                id: editTarget.id,
                data: { name, email, ...(password ? { password } : {}) },
              })
            }}
          >
            <div className="space-y-2">
              <Label htmlFor="edit-name">{t('admin.name')}</Label>
              <Input id="edit-name" name="name" defaultValue={editTarget?.name} required />
            </div>
            <div className="space-y-2">
              <Label htmlFor="edit-email">{t('admin.email')}</Label>
              <Input id="edit-email" name="email" type="email" defaultValue={editTarget?.email} required />
            </div>
            <div className="space-y-2">
              <Label htmlFor="edit-password">{t('admin.newPassword')}</Label>
              <Input
                id="edit-password"
                name="password"
                type="password"
                minLength={8}
                placeholder={t('admin.passwordHint')}
              />
            </div>
            {updateUser.isError && (
              <p className="text-sm text-destructive">{updateUser.error?.message}</p>
            )}
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setEditTarget(null)}>
                {t('admin.cancel')}
              </Button>
              <Button type="submit" disabled={updateUser.isPending}>
                {updateUser.isPending ? t('admin.saving') : t('admin.save')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={!!quotaTarget} onOpenChange={(open) => !open && setQuotaTarget(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t('admin.editQuota')}</DialogTitle>
            <DialogDescription>{quotaTarget?.email}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="quota-gb">{t('admin.quotaGb')}</Label>
              <Input
                id="quota-gb"
                type="number"
                min={0}
                step="any"
                value={quotaLimit}
                onChange={(e) => setQuotaLimit(e.target.value)}
                placeholder="0 = unlimited"
              />
            </div>
            {setUserQuota.isError && (
              <p className="text-sm text-destructive">{setUserQuota.error?.message}</p>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setQuotaTarget(null)}>
              {t('admin.cancel')}
            </Button>
            <Button
              disabled={setUserQuota.isPending}
              onClick={() => {
                if (!quotaTarget) return
                const gb = Number(quotaLimit)
                if (!Number.isFinite(gb) || gb < 0) return
                setUserQuota.mutate({ id: quotaTarget.id, limit: Math.round(gb * 1024 ** 3) })
              }}
            >
              {setUserQuota.isPending ? t('admin.saving') : t('admin.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('admin.deleteUserTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('admin.deleteUserConfirm')} <span className="font-medium">{deleteTarget?.name}</span>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('admin.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              disabled={deleteUser.isPending}
              onClick={() => deleteTarget && deleteUser.mutate(deleteTarget.id)}
            >
              {deleteUser.isPending ? t('admin.deleting') : t('admin.delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
