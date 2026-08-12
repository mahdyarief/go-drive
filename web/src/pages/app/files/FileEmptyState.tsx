import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { FolderOpen, Plus, Upload } from 'lucide-react'

interface FileEmptyStateProps {
  onNewFolder: () => void
  onUpload: (e: React.ChangeEvent<HTMLInputElement>) => void
  uploadPending: boolean
}

// FileEmptyState replaces the plain-text empty message with a large icon and
// quick actions so a brand-new folder is obviously actionable.
export function FileEmptyState({ onNewFolder, onUpload, uploadPending }: FileEmptyStateProps) {
  const { t } = useTranslation()

  return (
    <div className="flex flex-col items-center justify-center gap-4 py-16 text-center">
      <div className="flex h-16 w-16 items-center justify-center rounded-full bg-muted">
        <FolderOpen className="h-8 w-8 text-muted-foreground" />
      </div>
      <div>
        <p className="text-base font-medium">{t('files.empty')}</p>
        <p className="mt-1 text-sm text-muted-foreground">{t('files.emptyHint')}</p>
      </div>
      <div className="flex flex-wrap justify-center gap-2">
        <Button variant="outline" onClick={onNewFolder}>
          <Plus className="h-4 w-4 mr-2" />
          {t('files.newFolder')}
        </Button>
        <Button render={<label className="cursor-pointer flex items-center gap-1.5" />}>
          <Upload className="h-4 w-4 mr-2" />
          {t('files.upload')}
          <input type="file" className="hidden" onChange={onUpload} disabled={uploadPending} />
        </Button>
      </div>
    </div>
  )
}
