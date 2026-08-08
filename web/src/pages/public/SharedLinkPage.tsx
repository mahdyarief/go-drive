import { useState } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { useParams } from 'react-router'
import { api } from '@/lib/api'
import type { Folder, LockerFile } from '@/lib/types'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Download, File, Folder as FolderIcon, Loader2, Lock } from 'lucide-react'

interface PublicFileLink {
  type: 'file'
  name: string
  mimeType: string
  size: number
}

interface PublicFolderLink {
  type: 'folder'
  folders: Folder[]
  files: LockerFile[]
}

type PublicLinkData = PublicFileLink | PublicFolderLink

interface DownloadResult {
  url: string
  filename: string
  mimeType: string
  size: number
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(Math.floor(Math.log2(bytes) / 10), units.length - 1)
  return `${(bytes / 1024 ** i).toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

export default function SharedLinkPage() {
  const { token = '' } = useParams()
  const { t } = useTranslation()
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [downloadError, setDownloadError] = useState('')

  const info = useQuery({
    queryKey: ['shared', token],
    queryFn: () => api<PublicLinkData>(`/api/shared/${token}`),
  })

  const download = useMutation({
    mutationFn: (fileId?: string) => {
      const params = new URLSearchParams()
      if (fileId) params.set('fileId', fileId)
      if (password) params.set('password', password)
      const qs = params.toString()
      return api<DownloadResult>(`/api/shared/${token}/download${qs ? `?${qs}` : ''}`)
    },
    onSuccess: (data) => {
      setDownloadError('')
      window.open(data.url, '_blank', 'noopener,noreferrer')
    },
    onError: (err) => {
      const msg = err instanceof Error ? err.message : ''
      setDownloadError(msg)
      if (msg.includes('password')) {
        setShowPassword(true)
      }
    },
  })

  if (info.isPending) {
    return (
      <div className="mx-auto w-full max-w-2xl p-6">
        <p className="text-sm text-muted-foreground">{t('app.loading')}</p>
      </div>
    )
  }
  if (info.isError || !info.data) {
    return (
      <div className="mx-auto w-full max-w-2xl p-6">
        <Card>
          <CardContent className="pt-6">
            <p className="text-sm text-destructive">{t('public.linkNotFound')}</p>
          </CardContent>
        </Card>
      </div>
    )
  }

  const data = info.data

  return (
    <div className="mx-auto w-full max-w-2xl space-y-6 p-6">
      <header>
        <h1 className="text-2xl font-bold tracking-tight">{t('public.sharedTitle')}</h1>
        <p className="text-sm text-muted-foreground">{t('public.sharedSubtitle')}</p>
      </header>

      {showPassword && (
        <Card>
          <CardContent className="space-y-2 pt-6">
            <Label htmlFor="shared-password" className="flex items-center gap-2">
              <Lock className="h-3 w-3" />
              {t('public.enterPassword')}
            </Label>
            <Input
              id="shared-password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder={t('public.password')}
            />
          </CardContent>
        </Card>
      )}

      {downloadError && <p className="text-sm text-destructive">{downloadError}</p>}

      {data.type === 'file' ? (
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center gap-2 text-base">
              <File className="h-4 w-4 text-muted-foreground" />
              <span className="truncate">{data.name}</span>
            </CardTitle>
            <CardDescription>{formatBytes(data.size)}</CardDescription>
          </CardHeader>
          <CardContent>
            <Button disabled={download.isPending} onClick={() => download.mutate(undefined)}>
              {download.isPending && <Loader2 className="h-4 w-4 animate-spin mr-2" />}
              <Download className="h-4 w-4 mr-2" />
              {t('public.download')}
            </Button>
          </CardContent>
        </Card>
      ) : (
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center gap-2 text-base">
              <FolderIcon className="h-4 w-4 text-muted-foreground" />
              {t('public.folder')}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {data.folders.length === 0 && data.files.length === 0 && (
              <p className="text-sm text-muted-foreground">{t('public.emptyFolder')}</p>
            )}
            {data.folders.map((folder) => (
              <div key={folder.id} className="flex items-center gap-2 rounded-lg border p-3">
                <FolderIcon className="h-4 w-4 text-muted-foreground" />
                <span className="truncate text-sm">{folder.name}</span>
              </div>
            ))}
            {data.files.map((file) => (
              <div key={file.id} className="flex items-center justify-between gap-3 rounded-lg border p-3">
                <div className="flex min-w-0 items-center gap-2">
                  <File className="h-4 w-4 shrink-0 text-muted-foreground" />
                  <span className="truncate text-sm">{file.name}</span>
                  <span className="shrink-0 text-xs text-muted-foreground">{formatBytes(file.size)}</span>
                </div>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={download.isPending}
                  onClick={() => download.mutate(file.id)}
                >
                  {t('public.download')}
                </Button>
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  )
}
