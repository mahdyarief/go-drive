import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { LayoutGrid, List, Plus, Search, Upload } from 'lucide-react'
import type { ViewMode } from './files'

interface FileToolbarProps {
  viewMode: ViewMode
  onToggleView: (mode: ViewMode) => void
  searchValue: string
  onSearchChange: (value: string) => void
  onSearch: () => void
  onNewFolder: () => void
  onUpload: (e: React.ChangeEvent<HTMLInputElement>) => void
  uploadPending: boolean
}

// FileToolbar is the header action cluster of the files page: search box,
// list/grid view toggle, new folder, and upload button.
export function FileToolbar({
  viewMode,
  onToggleView,
  searchValue,
  onSearchChange,
  onSearch,
  onNewFolder,
  onUpload,
  uploadPending,
}: FileToolbarProps) {
  const { t } = useTranslation()

  return (
    <div className="flex items-center gap-2">
      <form
        className="relative"
        onSubmit={(e) => {
          e.preventDefault()
          onSearch()
        }}
      >
        <Search className="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          value={searchValue}
          onChange={(e) => onSearchChange(e.target.value)}
          placeholder={t('files.searchPlaceholder')}
          className="w-56 pl-8"
        />
      </form>
      <div className="flex items-center rounded-lg border p-0.5">
        <Button
          variant={viewMode === 'list' ? 'secondary' : 'ghost'}
          size="icon"
          className="h-8 w-8"
          aria-label={t('files.listView')}
          onClick={() => onToggleView('list')}
        >
          <List className="h-4 w-4" />
        </Button>
        <Button
          variant={viewMode === 'grid' ? 'secondary' : 'ghost'}
          size="icon"
          className="h-8 w-8"
          aria-label={t('files.gridView')}
          onClick={() => onToggleView('grid')}
        >
          <LayoutGrid className="h-4 w-4" />
        </Button>
      </div>
      <Button variant="outline" onClick={onNewFolder}>
        <Plus className="h-4 w-4 mr-2" />
        {t('files.newFolder')}
      </Button>
      <Button render={<label className="cursor-pointer flex items-center gap-1.5" />}>
        <Upload className="h-4 w-4" />
        {t('files.upload')}
        <input type="file" className="hidden" onChange={onUpload} disabled={uploadPending} />
      </Button>
    </div>
  )
}
