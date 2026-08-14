import { useState } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { useNavigate, useParams } from 'react-router'
import { tenantApi } from '@/lib/api'
import type { LockerFile, Tag } from '@/lib/types'
import { useOrgStore } from '@/store/org'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { ArrowLeft, Download, ExternalLink, Eye, File as FileIcon, Loader2 } from 'lucide-react'

interface DownloadUrlData {
  url: string
}

interface PreviewTokenData {
  token: string
  expiresAt: string
  url: string
}

interface FileTagsData {
  tags: Record<string, Tag[]>
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(Math.floor(Math.log2(bytes) / 10), units.length - 1)
  return `${(bytes / 1024 ** i).toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

const isPreviewableImage = (mimeType: string) => mimeType.startsWith('image/')
const isPreviewablePdf = (mimeType: string) => mimeType === 'application/pdf'

export default function FilePreviewPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { fileId = '' } = useParams()
  const currentOrg = useOrgStore((s) => s.currentOrg)
  const orgSlug = currentOrg?.slug
  const [previewUrl, setPreviewUrl] = useState('')

  const fileQuery = useQuery({
    queryKey: ['t', 'files', fileId, 'single', orgSlug],
    queryFn: () => tenantApi<{ file: LockerFile }>(`/api/t/files/${fileId}`, orgSlug!),
    enabled: !!orgSlug && !!fileId,
  })
  const file = fileQuery.data?.file

  const tagsQuery = useQuery({
    queryKey: ['t', 'files', fileId, 'tags', orgSlug],
    queryFn: () =>
      tenantApi<FileTagsData>(
        `/api/t/tags/for-files`,
        orgSlug!,
        { method: 'POST', body: JSON.stringify({ fileIds: [fileId] }) },
      ),
    enabled: !!orgSlug && !!fileId && !!fileQuery.data,
  })

  const downloadUrl = useMutation({
    mutationFn: () => tenantApi<DownloadUrlData>(`/api/t/files/${fileId}/download-url`, orgSlug!),
  })

  const previewToken = useMutation({
    mutationFn: () =>
      tenantApi<PreviewTokenData>(`/api/t/files/${fileId}/preview-token`, orgSlug!, {
        method: 'POST',
      }),
  })

  const handlePreview = () => {
    previewToken.mutate(undefined, {
      onSuccess: (data) => setPreviewUrl(window.location.origin + data.url),
    })
  }

  const handleDownload = () => {
    downloadUrl.mutate(undefined, {
      onSuccess: (data) => window.open(data.url, '_blank', 'noopener,noreferrer'),
    })
  }

  const handleOpenInNewTab = () => {
    previewToken.mutate(undefined, {
      onSuccess: (data) => {
        window.open(window.location.origin + data.url, '_blank', 'noopener,noreferrer')
      },
    })
  }

  if (fileQuery.isPending) {
    return (
      <div className="mx-auto w-full max-w-2xl space-y-6 p-6">
        <Button variant="outline" onClick={() => navigate('/app/files')}>
          <ArrowLeft className="h-4 w-4 mr-2" />
          {t('files.back')}
        </Button>
        <Card>
          <CardContent className="pt-6">
            <p className="flex items-center gap-2 text-sm text-muted-foreground">
              <Loader2 className="h-4 w-4 animate-spin" />
              {t('app.loading')}
            </p>
          </CardContent>
        </Card>
      </div>
    )
  }

  if (fileQuery.isError || !file) {
    return (
      <div className="mx-auto w-full max-w-2xl space-y-6 p-6">
        <Button variant="outline" onClick={() => navigate('/app/files')}>
          <ArrowLeft className="h-4 w-4 mr-2" />
          {t('files.back')}
        </Button>
        <Card>
          <CardContent className="pt-6">
            <p className="text-sm text-destructive">{t('files.previewNotFound')}</p>
          </CardContent>
        </Card>
      </div>
    )
  }

  const tags = tagsQuery.data?.tags?.[fileId] ?? []
  const showPreview = isPreviewableImage(file.mime_type) || isPreviewablePdf(file.mime_type)

  return (
    <div className="mx-auto w-full max-w-3xl space-y-6 p-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <Button variant="outline" onClick={() => navigate('/app/files')}>
          <ArrowLeft className="h-4 w-4 mr-2" />
          {t('files.back')}
        </Button>
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" onClick={handleOpenInNewTab} disabled={previewToken.isPending}>
            {previewToken.isPending && <Loader2 className="h-4 w-4 animate-spin mr-2" />}
            <ExternalLink className="h-4 w-4 mr-2" />
            {t('preview.openInNewTab')}
          </Button>
          <Button variant="outline" onClick={handleDownload} disabled={downloadUrl.isPending}>
            {downloadUrl.isPending && <Loader2 className="h-4 w-4 animate-spin mr-2" />}
            <Download className="h-4 w-4 mr-2" />
            {t('files.download')}
          </Button>
        </div>
      </div>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-base">
            <FileIcon className="h-4 w-4 text-muted-foreground" />
            <span className="truncate">{file.name}</span>
          </CardTitle>
          <CardDescription>
            {formatBytes(file.size)} · {file.mime_type || '—'}
          </CardDescription>
        </CardHeader>
        <Separator />
        <CardContent className="space-y-4 pt-4">
          {showPreview && (
            <div className="rounded-lg border bg-muted/50 p-4">
              {previewUrl ? (
                isPreviewablePdf(file.mime_type) ? (
                  <iframe
                    src={previewUrl}
                    title={file.name}
                    className="mx-auto h-[600px] w-full rounded-lg border"
                  />
                ) : (
                  <img
                    src={previewUrl}
                    alt={file.name}
                    className="mx-auto max-h-96 rounded-lg object-contain"
                  />
                )
              ) : (
                <div className="flex flex-col items-center gap-3 py-8">
                  <p className="text-sm text-muted-foreground">{t('files.previewHint')}</p>
                  <Button variant="outline" onClick={handlePreview} disabled={previewToken.isPending}>
                    {previewToken.isPending && <Loader2 className="h-4 w-4 animate-spin mr-2" />}
                    <Eye className="h-4 w-4 mr-2" />
                    {t('files.showPreview')}
                  </Button>
                </div>
              )}
            </div>
          )}

          <div className="space-y-2">
            <h3 className="text-sm font-medium">{t('files.details')}</h3>
            <div className="grid grid-cols-1 gap-2 text-sm sm:grid-cols-2">
              <div className="flex justify-between rounded-lg border px-3 py-2">
                <span className="text-muted-foreground">{t('files.size')}</span>
                <span>{formatBytes(file.size)}</span>
              </div>
              <div className="flex justify-between rounded-lg border px-3 py-2">
                <span className="text-muted-foreground">{t('files.type')}</span>
                <span className="truncate">{file.mime_type || '—'}</span>
              </div>
              <div className="flex justify-between rounded-lg border px-3 py-2">
                <span className="text-muted-foreground">{t('files.createdAt')}</span>
                <span>{new Date(file.created_at).toLocaleString()}</span>
              </div>
              <div className="flex justify-between rounded-lg border px-3 py-2">
                <span className="text-muted-foreground">{t('files.updatedAt')}</span>
                <span>{new Date(file.updated_at).toLocaleString()}</span>
              </div>
              <div className="flex justify-between rounded-lg border px-3 py-2">
                <span className="text-muted-foreground">{t('files.storageProvider')}</span>
                <span className="capitalize">{file.storage_provider || '—'}</span>
              </div>
              <div className="flex justify-between rounded-lg border px-3 py-2">
                <span className="text-muted-foreground">{t('files.status')}</span>
                <span>{file.status || '—'}</span>
              </div>
            </div>
          </div>

          {file.checksum && (
            <div className="space-y-1">
              <h3 className="text-sm font-medium">{t('files.checksum')}</h3>
              <p className="font-mono text-xs text-muted-foreground break-all">{file.checksum}</p>
            </div>
          )}

          <div className="space-y-2">
            <h3 className="text-sm font-medium">{t('files.tags')}</h3>
            {tags.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t('files.noTags')}</p>
            ) : (
              <div className="flex flex-wrap gap-2">
                {tags.map((tag) => (
                  <Badge key={tag.id} variant="secondary">
                    {tag.name}
                  </Badge>
                ))}
              </div>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
