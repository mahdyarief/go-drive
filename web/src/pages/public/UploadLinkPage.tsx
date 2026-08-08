import { useRef, useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { useParams } from 'react-router'
import { api } from '@/lib/api'
import type { LockerFile } from '@/lib/types'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { CheckCircle2, Loader2, Lock, Upload } from 'lucide-react'

interface UploadResult {
  file: LockerFile
}

export default function UploadLinkPage() {
  const { token = '' } = useParams()
  const { t } = useTranslation()
  const [file, setFile] = useState<File | null>(null)
  const [password, setPassword] = useState('')
  const fileInputRef = useRef<HTMLInputElement>(null)

  const upload = useMutation({
    mutationFn: (f: File) => {
      const form = new FormData()
      form.append('token', token)
      form.append('file', f)
      // Server reads the link password only from X-Link-Password or ?password=
      // (linkPassword in share.go) — never from the multipart body.
      const params = new URLSearchParams()
      if (password) params.set('password', password)
      const qs = params.toString()
      return api<UploadResult>(`/api/upload/public${qs ? `?${qs}` : ''}`, { method: 'POST', body: form })
    },
    onSuccess: () => {
      setFile(null)
      if (fileInputRef.current) fileInputRef.current.value = ''
    },
  })

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const selected = e.target.files
    setFile(selected?.[0] ?? null)
  }

  return (
    <div className="mx-auto w-full max-w-2xl space-y-6 p-6">
      <header>
        <h1 className="text-2xl font-bold tracking-tight">{t('public.uploadTitle')}</h1>
        <p className="text-sm text-muted-foreground">{t('public.uploadSubtitle')}</p>
      </header>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">{t('public.upload')}</CardTitle>
          <CardDescription>{t('public.chooseFile')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <Input ref={fileInputRef} type="file" onChange={handleFileChange} />
          <div className="space-y-2">
            <Label htmlFor="upload-password" className="flex items-center gap-2">
              <Lock className="h-3 w-3" />
              {t('public.passwordOptional')}
            </Label>
            <Input id="upload-password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
          </div>
          <Button disabled={!file || upload.isPending} onClick={() => file && upload.mutate(file)}>
            {upload.isPending && <Loader2 className="h-4 w-4 animate-spin mr-2" />}
            <Upload className="h-4 w-4 mr-2" />
            {upload.isPending ? t('public.uploading') : t('public.upload')}
          </Button>
          {upload.isSuccess && upload.data && (
            <p className="flex items-center gap-2 text-sm text-emerald-600">
              <CheckCircle2 className="h-4 w-4" />
              {t('public.uploadSuccess', { name: upload.data.file.name })}
            </p>
          )}
          {upload.isError && (
            <p className="text-sm text-destructive">
              {upload.error instanceof Error ? upload.error.message : t('public.uploadFailed')}
            </p>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
