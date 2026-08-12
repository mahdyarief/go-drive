import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { CheckCircle2, ChevronDown, ChevronUp, Loader2, X, XCircle } from 'lucide-react'
import { useUploadStore } from '@/store/upload'

// UploadPanel is a fixed bottom-right progress panel fed by the global upload
// store. It appears on every app page while uploads are in flight and lists
// one row per file with status; rows can be dismissed and the list cleared.
export function UploadPanel() {
  const { t } = useTranslation()
  const entries = useUploadStore((s) => s.entries)
  const remove = useUploadStore((s) => s.remove)
  const clear = useUploadStore((s) => s.clear)
  const [open, setOpen] = useState(true)

  if (entries.length === 0) return null

  return (
    <div className="fixed bottom-4 right-4 z-50 w-80">
      <div className="rounded-lg border bg-background shadow-lg">
        <div className="flex items-center justify-between border-b px-3 py-2">
          <p className="text-sm font-medium">{t('files.uploads', { count: entries.length })}</p>
          <div className="flex items-center gap-1">
            <Button
              variant="ghost"
              size="icon"
              className="h-6 w-6"
              onClick={() => setOpen(!open)}
              aria-label={open ? t('files.collapse') : t('files.expand')}
            >
              {open ? <ChevronDown className="h-4 w-4" /> : <ChevronUp className="h-4 w-4" />}
            </Button>
            <Button variant="ghost" size="icon" className="h-6 w-6" onClick={clear} aria-label={t('files.clearUploads')}>
              <X className="h-4 w-4" />
            </Button>
          </div>
        </div>
        {open && (
          <div className="space-y-2 p-3">
            {entries.map((entry) => (
              <div key={entry.id} className="flex items-center gap-2 text-xs">
                {entry.status === 'uploading' && <Loader2 className="h-3.5 w-3.5 animate-spin shrink-0 text-muted-foreground" />}
                {entry.status === 'done' && <CheckCircle2 className="h-3.5 w-3.5 shrink-0 text-emerald-500" />}
                {entry.status === 'error' && <XCircle className="h-3.5 w-3.5 shrink-0 text-destructive" />}
                <span className="min-w-0 flex-1 truncate">{entry.name}</span>
                {entry.status === 'uploading' && <span className="text-muted-foreground">{entry.percent}%</span>}
                <Button variant="ghost" size="icon" className="h-5 w-5" onClick={() => remove(entry.id)} aria-label={t('files.dismiss')}>
                  <X className="h-3 w-3" />
                </Button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
