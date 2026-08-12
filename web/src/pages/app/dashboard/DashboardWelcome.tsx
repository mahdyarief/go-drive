import { useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { Button } from '@/components/ui/button'
import { useAuthStore } from '@/store/auth'
import { useOrgStore } from '@/store/org'
import { useUploadStore } from '@/store/upload'
import { FolderPlus, Link2, Upload } from 'lucide-react'

// DashboardWelcome greets the user and offers quick actions: upload files to
// the workspace root, or jump straight into the files/links sections.
export function DashboardWelcome() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const user = useAuthStore((s) => s.user)
  const currentOrg = useOrgStore((s) => s.currentOrg)
  const orgSlug = currentOrg?.slug
  const uploadBatch = useUploadStore((s) => s.uploadBatch)
  const inputRef = useRef<HTMLInputElement>(null)

  const handleUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files ?? [])
    if (files.length > 0 && orgSlug) {
      uploadBatch(files, orgSlug, null)
    }
    e.target.value = ''
  }

  return (
    <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">
          {t('dashboard.welcome', { name: user?.name || user?.email })}
        </h1>
        <p className="text-sm text-muted-foreground">
          {currentOrg ? `${currentOrg.name} · ${t(`org.role.${currentOrg.role}`)}` : t('dashboard.subtitle')}
        </p>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <input ref={inputRef} type="file" multiple className="hidden" onChange={handleUpload} aria-label={t('dashboard.uploadFiles')} />
        <Button size="sm" onClick={() => inputRef.current?.click()}>
          <Upload className="h-4 w-4" />
          {t('dashboard.uploadFiles')}
        </Button>
        <Button size="sm" variant="outline" onClick={() => navigate('/app/files')}>
          <FolderPlus className="h-4 w-4" />
          {t('dashboard.newFolder')}
        </Button>
        <Button size="sm" variant="outline" onClick={() => navigate('/app/links')}>
          <Link2 className="h-4 w-4" />
          {t('dashboard.openLinks')}
        </Button>
      </div>
    </div>
  )
}
