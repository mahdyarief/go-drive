import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { FileText, X } from 'lucide-react'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { compressToWebP } from '@/lib/imageCompress'

interface FilePickerProps {
  value: File | null
  onChange: (file: File | null) => void
  accept?: string
  label?: string
}

export function FilePicker({ value, onChange, accept = 'image/*,application/pdf', label }: FilePickerProps) {
  const { t } = useTranslation()
  const [previewUrl, setPreviewUrl] = useState<string | null>(null)

  const handleChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0] ?? null
    if (previewUrl) URL.revokeObjectURL(previewUrl)
    if (!file) {
      onChange(null)
      setPreviewUrl(null)
      return
    }
    const processed = await compressToWebP(file)
    onChange(processed)
    setPreviewUrl(processed.type === 'application/pdf' ? null : URL.createObjectURL(processed))
  }

  const handleClear = () => {
    onChange(null)
    if (previewUrl) URL.revokeObjectURL(previewUrl)
    setPreviewUrl(null)
  }

  const isPdf = value?.type === 'application/pdf'

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2">
        <Input type="file" accept={accept} onChange={handleChange} className="cursor-pointer" aria-label={label} />
        {value && (
          <Button type="button" variant="ghost" size="icon" onClick={handleClear} aria-label={t('order.removeProof')}>
            <X className="h-4 w-4" />
          </Button>
        )}
      </div>
      {value && isPdf && (
        <div className="flex h-20 items-center gap-2 rounded-md border border-border px-3 text-sm text-muted-foreground">
          <FileText className="h-5 w-5 shrink-0" />
          <span className="truncate">{value.name}</span>
        </div>
      )}
      {value && !isPdf && previewUrl && (
        <img src={previewUrl} alt={label ?? 'preview'} className="h-20 w-auto rounded-md border border-border object-cover" />
      )}
    </div>
  )
}
