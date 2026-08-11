import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { tenantApi } from '@/lib/api'
import type { Store } from '@/lib/types'
import { Button, buttonVariants } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
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
import { ExternalLink, HelpCircle, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import type { IngestMode, Provider, StoreForm, WriteMode } from './stores'

interface StoreFormDialogProps {
  orgSlug: string | undefined
  createOpen: boolean
  setCreateOpen: (open: boolean) => void
  editTarget: Store | null
  setEditTarget: (store: Store | null) => void
  deleteTarget: Store | null
  setDeleteTarget: (store: Store | null) => void
  form: StoreForm
  setForm: (form: StoreForm) => void
  showLocalHelp: boolean
  setShowLocalHelp: (open: boolean) => void
  showGdriveHelp: boolean
  setShowGdriveHelp: (open: boolean) => void
  gdriveRedirectUri?: string
}

// StoreFormDialog owns the attach/edit store form dialog plus the local
// storage help, Google Drive credentials help, and delete-store confirm
// dialogs so the page keeps only page-level concerns.
export function StoreFormDialog(props: StoreFormDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { orgSlug, createOpen, setCreateOpen, editTarget, setEditTarget, deleteTarget, setDeleteTarget, form, setForm, showLocalHelp, setShowLocalHelp, showGdriveHelp, setShowGdriveHelp, gdriveRedirectUri } = props

  const invalidateStores = () => {
    queryClient.invalidateQueries({ queryKey: ['t', 'stores', orgSlug] })
    queryClient.invalidateQueries({ queryKey: ['t', 'stores', 'sync', orgSlug] })
  }

  const setConfigField = (key: string, value: string) => {
    setForm({ ...form, config: { ...form.config, [key]: value } })
  }

  const setCredentialField = (key: string, value: string) => {
    setForm({ ...form, credentials: { ...form.credentials, [key]: value } })
  }

  const createStore = () =>
    tenantApi<{ store: Store }>('/api/t/stores', orgSlug!, {
      method: 'POST',
      body: JSON.stringify({
        name: form.name,
        provider: form.provider,
        writeMode: form.writeMode,
        ingestMode: form.ingestMode,
        readPriority: form.readPriority,
        quotaLimit: form.quotaLimit > 0 ? Math.round(form.quotaLimit * 1024 ** 3) : 0,
        config: form.config,
        credentials: form.credentials,
      }),
    })

  const updateStore = (id: string) =>
    tenantApi<{ store: Store }>(`/api/t/stores/${id}`, orgSlug!, {
      method: 'PATCH',
      body: JSON.stringify({
        name: form.name,
        provider: form.provider,
        writeMode: form.writeMode,
        ingestMode: form.ingestMode,
        readPriority: form.readPriority,
        quotaLimit: form.quotaLimit > 0 ? Math.round(form.quotaLimit * 1024 ** 3) : 0,
        config: form.config,
      }),
    })

  const deleteStore = (id: string) => tenantApi<unknown>(`/api/t/stores/${id}`, orgSlug!, { method: 'DELETE' })

  const [createPending, setCreatePending] = useState(false)
  const [updatePending, setUpdatePending] = useState(false)
  const [deletePending, setDeletePending] = useState(false)

  const submit = async () => {
    if (editTarget) {
      setUpdatePending(true)
      try {
        await updateStore(editTarget.id)
        invalidateStores()
        toast.success(t('stores.storeUpdated'))
        setCreateOpen(false)
        setEditTarget(null)
      } finally {
        setUpdatePending(false)
      }
    } else {
      setCreatePending(true)
      try {
        await createStore()
        invalidateStores()
        setForm({ ...form, name: '' })
        setCreateOpen(false)
      } finally {
        setCreatePending(false)
      }
    }
  }

  const confirmDelete = async () => {
    if (!deleteTarget) return
    setDeletePending(true)
    try {
      await deleteStore(deleteTarget.id)
      invalidateStores()
      setDeleteTarget(null)
    } finally {
      setDeletePending(false)
    }
  }

  return (
    <>
      {/* Attach store dialog */}
      <Dialog
        open={createOpen}
        onOpenChange={(open) => {
          setCreateOpen(open)
          if (!open) setEditTarget(null)
        }}
      >
        <DialogContent className="max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{editTarget ? t('stores.editStore') : t('stores.createStore')}</DialogTitle>
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
                <Select value={form.provider} onValueChange={(v) => setForm({ ...form, provider: v as Provider })} disabled={!!editTarget}>
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
              <Select value={form.writeMode} onValueChange={(v) => setForm({ ...form, writeMode: v as WriteMode })}>
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
              <Select value={form.ingestMode} onValueChange={(v) => setForm({ ...form, ingestMode: v as IngestMode })}>
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

            <div className="space-y-2">
              <Label htmlFor="store-quota">{t('stores.quotaLimit')}</Label>
              <Input
                id="store-quota"
                type="number"
                min={0}
                placeholder={t('stores.quotaLimitHint')}
                value={form.quotaLimit || ''}
                onChange={(e) => setForm({ ...form, quotaLimit: Number(e.target.value) })}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setCreateOpen(false)
                setEditTarget(null)
              }}
            >
              {t('links.cancel')}
            </Button>
            <Button
              disabled={!form.name || createPending || updatePending}
              onClick={() => void submit()}
            >
              {(createPending || updatePending) && <Loader2 className="h-4 w-4 animate-spin mr-2" />}
              {t(editTarget ? 'stores.editStore' : 'stores.createStore')}
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
            <AlertDialogAction variant="destructive" disabled={deletePending} onClick={() => void confirmDelete()}>
              {t('stores.deleteStore')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
