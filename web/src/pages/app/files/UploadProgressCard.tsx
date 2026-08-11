import { useTranslation } from 'react-i18next'
import { Card, CardContent } from '@/components/ui/card'

export interface UploadProgress {
  name: string
  percent: number
}

interface UploadProgressCardProps {
  progress: UploadProgress | null
}

// UploadProgressCard shows the current upload as a slim card while one is in
// flight; renders nothing otherwise.
export function UploadProgressCard({ progress }: UploadProgressCardProps) {
  const { t } = useTranslation()

  if (!progress) return null

  return (
    <Card>
      <CardContent className="py-3 text-sm">
        <p className="text-muted-foreground">
          {t('files.uploadProgress', { name: progress.name, percent: progress.percent })}
        </p>
      </CardContent>
    </Card>
  )
}
