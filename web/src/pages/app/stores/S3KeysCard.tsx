import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { tenantApi } from '@/lib/api'
import type { S3Key } from '@/lib/types'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
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
import { HelpCircle, KeyRound, Loader2, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { copyText } from './stores'
import type { CreateKeyData, KeysData } from './stores'

interface S3KeysCardProps {
  orgSlug: string | undefined
  s3Endpoint: string
  isDev: boolean
}

// buildAiPrompt is the single source of truth for the AI agent integration
// block — used both by the rendered <pre> and the copy button so they can
// never drift apart. The dev-only port note is appended only in development;
// self-hosted deployments must not be told to use the :8081/:5173 split.
function buildAiPrompt(s3Endpoint: string, isDev: boolean): string {
  const devNote = isDev
    ? 'Note: in development use the API port :8081, not the Vite proxy :5173\n(the proxy rewrites Host and breaks SigV4).'
    : ''
  return `S3 endpoint: ${s3Endpoint}
Access key ID: <your access key>
Secret access key: <your secret>
Region: us-east-1
Auth: AWS SigV4

Supported operations: ListObjectsV2 (list + prefix), GetObject,
HeadObject, PutObject (folders auto-created via key path), DeleteObject,
multipart upload (CreateMultipartUpload, UploadPart, CompleteMultipartUpload,
AbortMultipartUpload, ListParts, ListMultipartUploads).

To use with AWS CLI:
aws --endpoint-url ${s3Endpoint} s3 ls s3://
aws --endpoint-url ${s3Endpoint} s3 cp file.txt s3://hello.txt

To use with rclone: type=s3, provider=Other, endpoint=${s3Endpoint}.
${devNote}`
}

// S3KeysCard renders the S3 API keys card plus its dialogs (create key, key
// created, delete key, connect guide with AI agent prompt). All S3-key data
// fetching and mutations live here so the page stays small.
export function S3KeysCard({ orgSlug, s3Endpoint, isDev }: S3KeysCardProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const [keyName, setKeyName] = useState('')
  const [keyPermissions, setKeyPermissions] = useState('readwrite')
  const [createKeyOpen, setCreateKeyOpen] = useState(false)
  const [keyCreatedData, setKeyCreatedData] = useState<CreateKeyData | null>(null)
  const [deleteKeyTarget, setDeleteKeyTarget] = useState<S3Key | null>(null)
  const [showS3Help, setShowS3Help] = useState(false)

  const keysQuery = useQuery({
    queryKey: ['t', 's3keys', orgSlug],
    queryFn: () => tenantApi<KeysData>('/api/t/s3-keys', orgSlug!),
    enabled: !!orgSlug,
  })

  const createKey = useMutation({
    mutationFn: () =>
      tenantApi<CreateKeyData>('/api/t/s3-keys', orgSlug!, {
        method: 'POST',
        body: JSON.stringify({ name: keyName, permissions: keyPermissions }),
      }),
    onSuccess: (data) => {
      setKeyCreatedData(data)
      setCreateKeyOpen(false)
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

  const keys = keysQuery.data?.keys ?? []

  // copyAiPrompt copies a ready-to-send instruction block so the user can
  // paste it into their AI agent to integrate with the S3 gateway.
  const copyAiPrompt = async () => {
    const ok = await copyText(buildAiPrompt(s3Endpoint, isDev))
    if (!ok) toast.error(t('stores.copyFailed'))
  }

  // copyWithFeedback copies text and reports a failure toast when the
  // clipboard write is blocked by the browser.
  const copyWithFeedback = async (text: string) => {
    const ok = await copyText(text)
    if (!ok) toast.error(t('stores.copyFailed'))
  }

  return (
    <>
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
          <Button variant="ghost" size="sm" className="mt-1" onClick={() => setShowS3Help(true)}>
            <HelpCircle className="h-3.5 w-3.5 mr-1" />
            {t('stores.s3ConnectTrigger')}
          </Button>
        </CardContent>
      </Card>

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
              <Select value={keyPermissions} onValueChange={(v) => setKeyPermissions(v ?? 'readwrite')}>
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
            <Button disabled={!keyName.trim() || createKey.isPending} onClick={() => createKey.mutate()}>
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
                  <Button variant="outline" onClick={() => void copyWithFeedback(keyCreatedData.accessKeyId)}>
                    {t('stores.copyAccessKey')}
                  </Button>
                </div>
              </div>
              <div className="space-y-2">
                <Label>{t('stores.secretAccessKey')}</Label>
                <div className="flex gap-2">
                  <Input readOnly value={keyCreatedData.secretAccessKey} />
                  <Button variant="outline" onClick={() => void copyWithFeedback(keyCreatedData.secretAccessKey)}>
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

      {/* S3 connect guide dialog */}
      <Dialog open={showS3Help} onOpenChange={setShowS3Help}>
        <DialogContent className="sm:max-w-2xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{t('stores.s3ConnectTitle')}</DialogTitle>
            <DialogDescription>{t('stores.s3ConnectSubtitle')}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 text-sm">
            <ol className="list-decimal space-y-1 pl-4">
              <li>{t('stores.s3ConnectStep1')}</li>
              <li>{t('stores.s3ConnectStep2')}</li>
            </ol>
            <div className="space-y-2">
              <Label>{t('stores.s3ConnectEndpoint')}</Label>
              <div className="flex gap-2">
                <Input readOnly value={s3Endpoint} className="font-mono text-xs" />
                <Button variant="outline" onClick={() => void copyWithFeedback(s3Endpoint)}>
                  {t('stores.s3ConnectCopyEndpoint')}
                </Button>
              </div>
            </div>
            <div className="space-y-2">
              <p className="font-medium">{t('stores.s3ConnectFeatures')}</p>
              <ul className="list-disc space-y-1 pl-4 text-muted-foreground">
                <li>{t('stores.s3ConnectFeatureFolders')}</li>
                <li>{t('stores.s3ConnectFeatureList')}</li>
                <li>{t('stores.s3ConnectFeatureUpload')}</li>
                <li>{t('stores.s3ConnectFeatureDownload')}</li>
                <li>{t('stores.s3ConnectFeatureDelete')}</li>
                <li>{t('stores.s3ConnectFeatureMultipart')}</li>
              </ul>
            </div>
            <div className="space-y-2">
              <p className="font-medium">{t('stores.s3ConnectAwsCli')}</p>
              <p className="text-xs text-muted-foreground">{t('stores.s3ConnectAwsCliNote')}</p>
              <pre className="overflow-x-auto rounded-md bg-muted p-3 font-mono text-xs">{`aws configure
aws --endpoint-url ${s3Endpoint} s3 ls s3://
aws --endpoint-url ${s3Endpoint} s3 cp file.txt s3://hello.txt
aws --endpoint-url ${s3Endpoint} s3 cp s3://hello.txt file.txt`}</pre>
            </div>
            <div className="space-y-2">
              <p className="font-medium">{t('stores.s3ConnectRclone')}</p>
              <p className="text-xs text-muted-foreground">{t('stores.s3ConnectRcloneNote')}</p>
              <pre className="overflow-x-auto rounded-md bg-muted p-3 font-mono text-xs">{`rclone config
  type = s3
  provider = Other
  endpoint = ${s3Endpoint}
  access_key_id = <access key id>
  secret_access_key = <secret access key>
  region = us-east-1

rclone ls <remote>:
rclone copy file.txt <remote>:
rclone copy <remote>:hello.txt ./`}</pre>
            </div>
            {isDev && <p className="text-xs text-muted-foreground">{t('stores.s3ConnectDevNote')}</p>}
            <p className="text-xs">
              {t('stores.s3ConnectDocsNote')}{' '}
              <a href="https://docs.aws.amazon.com/AmazonS3/latest/userguide/Welcome.html" target="_blank" rel="noreferrer" className="text-primary underline">
                {t('stores.s3ConnectDocsLink')}
              </a>
            </p>
            <div className="space-y-2">
              <p className="font-medium">{t('stores.s3ConnectAiAgent')}</p>
              <p className="text-xs text-muted-foreground">{t('stores.s3ConnectAiAgentNote')}</p>
              <pre className="overflow-x-auto whitespace-pre-wrap rounded-md bg-muted p-3 font-mono text-xs">{buildAiPrompt(s3Endpoint, isDev)}</pre>
              <Button variant="outline" size="sm" onClick={() => copyAiPrompt()}>
                {t('stores.s3ConnectCopyAiPrompt')}
              </Button>
            </div>
          </div>
          <DialogFooter>
            <Button onClick={() => setShowS3Help(false)}>{t('links.save')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
