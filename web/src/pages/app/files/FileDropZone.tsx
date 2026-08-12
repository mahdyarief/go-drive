import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

interface FileDropZoneProps {
  onFiles: (files: FileList) => void
  children: React.ReactNode
}

const handleDragOver = (e: React.DragEvent) => {
  e.preventDefault()
}

// FileDropZone wraps the files page and turns the whole area into an upload
// drop target. The dragDepth counter keeps the overlay visible while the
// pointer moves across child elements that fire their own dragenter/leave.
export function FileDropZone({ onFiles, children }: FileDropZoneProps) {
  const { t } = useTranslation()
  const [isDragging, setIsDragging] = useState(false)
  const dragDepth = useRef(0)

  const handleDragEnter = (e: React.DragEvent) => {
    e.preventDefault()
    dragDepth.current += 1
    setIsDragging(true)
  }

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault()
    dragDepth.current -= 1
    if (dragDepth.current <= 0) {
      dragDepth.current = 0
      setIsDragging(false)
    }
  }

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault()
    dragDepth.current = 0
    setIsDragging(false)
    const dropped = e.dataTransfer.files
    if (dropped.length > 0) onFiles(dropped)
  }

  return (
    <div
      className="relative"
      onDragEnter={handleDragEnter}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
    >
      {isDragging && (
        <div className="pointer-events-none absolute inset-0 z-20 flex items-center justify-center rounded-xl border-2 border-dashed border-primary bg-background/80">
          <p className="text-sm font-medium">{t('files.dropFiles')}</p>
        </div>
      )}
      {children}
    </div>
  )
}
