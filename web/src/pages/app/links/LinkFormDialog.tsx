import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import type { LinkForm, LinkKind } from './links'

interface LinkFormDialogProps {
  open: boolean
  kind: LinkKind
  editTarget: { kind: LinkKind; id: string } | null
  form: LinkForm
  onFormChange: (form: LinkForm) => void
  isPending: boolean
  onSave: () => void
  onCancel: () => void
}

export function LinkFormDialog({ open, kind, editTarget, form, onFormChange, isPending, onSave, onCancel }: LinkFormDialogProps) {
  const { t } = useTranslation()
  const set = (patch: Partial<LinkForm>) => onFormChange({ ...form, ...patch })
  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onCancel() }}>
      <DialogContent className="max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{editTarget ? t('links.edit') : t('links.create')}</DialogTitle>
          <DialogDescription>
            {editTarget ? t('links.edit') : t('links.create')} — {kind === 'share' ? t('links.tabShare') : kind === 'upload' ? t('links.tabUpload') : t('links.tabTracked')}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          {kind !== 'share' && (
            <div className="space-y-2">
              <Label htmlFor="l-name">{t('links.name')}</Label>
              <Input id="l-name" value={form.name} onChange={(e) => set({ name: e.target.value })} />
            </div>
          )}
          {kind !== 'upload' && (
            <div className="space-y-2">
              <Label htmlFor="l-access">{t('links.access')}</Label>
              <Select value={form.access} onValueChange={(v) => set({ access: v ?? 'view' })}>
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
          {kind !== 'upload' && (
            <div className="space-y-2">
              <Label htmlFor="l-file">{t('links.fileId')}</Label>
              <Input id="l-file" value={form.fileId} onChange={(e) => set({ fileId: e.target.value })} placeholder="uuid" />
            </div>
          )}
          <div className="space-y-2">
            <Label htmlFor="l-folder">{t('links.folderId')}</Label>
            <Input id="l-folder" value={form.folderId} onChange={(e) => set({ folderId: e.target.value })} placeholder="uuid" />
          </div>
          {kind === 'tracked' && (
            <div className="space-y-2">
              <Label htmlFor="l-desc">{t('links.description')}</Label>
              <Input id="l-desc" value={form.description} onChange={(e) => set({ description: e.target.value })} />
            </div>
          )}
          <div className="space-y-2">
            <Label htmlFor="l-password">{t('links.password')}</Label>
            <Input id="l-password" type="password" value={form.password} onChange={(e) => set({ password: e.target.value })} placeholder={t('links.passwordHint')} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="l-expires">{t('links.expires')}</Label>
            <Input id="l-expires" type="date" value={form.expiresAt} onChange={(e) => set({ expiresAt: e.target.value })} />
          </div>
          {kind === 'share' && (
            <div className="space-y-2">
              <Label htmlFor="l-maxdl">{t('links.maxDownloads')}</Label>
              <Input id="l-maxdl" type="number" value={form.maxDownloads} onChange={(e) => set({ maxDownloads: e.target.value })} />
            </div>
          )}
          {kind === 'upload' && (
            <>
              <div className="space-y-2">
                <Label htmlFor="l-maxfiles">{t('links.maxFiles')}</Label>
                <Input id="l-maxfiles" type="number" value={form.maxFiles} onChange={(e) => set({ maxFiles: e.target.value })} />
              </div>
              <div className="space-y-2">
                <Label htmlFor="l-maxsize">{t('links.maxFileSize')}</Label>
                <Input id="l-maxsize" type="number" value={form.maxFileSize} onChange={(e) => set({ maxFileSize: e.target.value })} />
              </div>
            </>
          )}
          {kind === 'tracked' && (
            <>
              <div className="space-y-2">
                <Label htmlFor="l-maxviews">{t('links.maxViews')}</Label>
                <Input id="l-maxviews" type="number" value={form.maxViews} onChange={(e) => set({ maxViews: e.target.value })} />
              </div>
              <div className="flex items-center gap-2">
                <Switch id="l-email" checked={form.requireEmail} onCheckedChange={(v) => set({ requireEmail: v })} />
                <Label htmlFor="l-email">{t('links.requireEmail')}</Label>
              </div>
            </>
          )}
          {editTarget && (
            <div className="flex items-center gap-2">
              <Switch id="l-active" checked={form.isActive} onCheckedChange={(v) => set({ isActive: v })} />
              <Label htmlFor="l-active">{t('links.status')}</Label>
            </div>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onCancel}>
            {t('links.cancel')}
          </Button>
          <Button
            disabled={isPending || (kind !== 'share' && !form.name)}
            onClick={onSave}
          >
            {t('links.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
