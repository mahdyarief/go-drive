import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api } from '@/lib/api'
import type { OrgDetailsData } from '@/lib/types'
import { useOrgStore } from '@/store/org'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Loader2, Plus, Trash2, UserRound } from 'lucide-react'

const roleLabel = (role: string, t: (k: string) => string) => {
  if (role === 'owner') return t('org.role.owner')
  if (role === 'admin') return t('org.role.admin')
  return t('org.role.member')
}

export default function SettingsMembersPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const currentOrg = useOrgStore((s) => s.currentOrg)
  const orgSlug = currentOrg?.slug

  const [addOpen, setAddOpen] = useState(false)
  const [userId, setUserId] = useState('')
  const [newRole, setNewRole] = useState('member')

  const orgQuery = useQuery({
    queryKey: ['org', orgSlug],
    queryFn: () => api<OrgDetailsData>(`/api/orgs/${orgSlug}`),
    enabled: !!orgSlug,
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['org', orgSlug] })
  }

  const addMember = useMutation({
    mutationFn: () =>
      api<unknown>(`/api/orgs/${orgSlug}/members`, {
        method: 'POST',
        body: JSON.stringify({ user_id: userId.trim(), role: newRole }),
      }),
    onSuccess: () => {
      invalidate()
      setAddOpen(false)
      setUserId('')
      setNewRole('member')
    },
  })

  const removeMember = useMutation({
    mutationFn: (memberUserId: string) =>
      api<unknown>(`/api/orgs/${orgSlug}/members/${memberUserId}`, { method: 'DELETE' }),
    onSuccess: invalidate,
  })

  const changeRole = useMutation({
    mutationFn: ({ memberUserId, role }: { memberUserId: string; role: string }) =>
      api<unknown>(`/api/orgs/${orgSlug}/members/${memberUserId}`, {
        method: 'PATCH',
        body: JSON.stringify({ role }),
      }),
    onSuccess: invalidate,
  })

  const members = orgQuery.data?.members ?? []
  const yourRole = orgQuery.data?.your_role ?? ''
  const canManage = yourRole === 'owner' || yourRole === 'admin'
  const canChangeRoles = yourRole === 'owner'

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t('settings.members.title')}</h1>
          <p className="text-sm text-muted-foreground">{t('settings.members.description')}</p>
        </div>
        {canManage && (
          <Button onClick={() => setAddOpen(true)}>
            <Plus className="h-4 w-4 mr-2" />
            {t('settings.members.add')}
          </Button>
        )}
      </div>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">{t('settings.members.list')}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          {orgQuery.isPending && (
            <p className="text-sm text-muted-foreground py-8 text-center">...</p>
          )}
          {orgQuery.isError && (
            <p className="text-sm text-destructive py-8 text-center">{t('settings.members.loadError')}</p>
          )}
          {!orgQuery.isPending && !orgQuery.isError && members.length === 0 && (
            <p className="text-sm text-muted-foreground py-8 text-center">{t('settings.members.empty')}</p>
          )}
          {members.map((member) => (
            <div key={member.id} className="flex items-center justify-between gap-3 rounded-lg border p-3">
              <div className="flex min-w-0 items-center gap-2">
                <UserRound className="h-4 w-4 shrink-0 text-muted-foreground" />
                <span className="truncate text-sm font-mono">{member.user_id}</span>
              </div>
              <div className="flex shrink-0 items-center gap-2">
                <Badge variant="secondary">{roleLabel(member.role, t)}</Badge>
                {canChangeRoles && member.role !== 'owner' && (
                  <Select
                    value={member.role}
                    onValueChange={(v) => changeRole.mutate({ memberUserId: member.user_id, role: v ?? member.role })}
                  >
                    <SelectTrigger className="h-8 w-28" aria-label={t('settings.members.role')}>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="admin">{t('org.role.admin')}</SelectItem>
                      <SelectItem value="member">{t('org.role.member')}</SelectItem>
                    </SelectContent>
                  </Select>
                )}
                {canManage && member.role !== 'owner' && (
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8 text-destructive"
                    aria-label={t('settings.members.remove')}
                    disabled={removeMember.isPending}
                    onClick={() => removeMember.mutate(member.user_id)}
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                )}
              </div>
            </div>
          ))}
        </CardContent>
      </Card>

      <Dialog open={addOpen} onOpenChange={(o) => { if (!o) { setAddOpen(false); setUserId(''); setNewRole('member') } }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('settings.members.add')}</DialogTitle>
            <DialogDescription>{t('settings.members.addHint')}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="member-user-id">{t('settings.members.userId')}</Label>
              <Input
                id="member-user-id"
                value={userId}
                onChange={(e) => setUserId(e.target.value)}
                placeholder="uuid"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="member-role">{t('settings.members.role')}</Label>
              <Select value={newRole} onValueChange={(v) => setNewRole(v ?? 'member')}>
                <SelectTrigger id="member-role">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="admin">{t('org.role.admin')}</SelectItem>
                  <SelectItem value="member">{t('org.role.member')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setAddOpen(false)}>
              {t('links.cancel')}
            </Button>
            <Button
              disabled={!userId.trim() || addMember.isPending}
              onClick={() => addMember.mutate()}
            >
              {addMember.isPending && <Loader2 className="h-4 w-4 animate-spin mr-2" />}
              {t('links.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
