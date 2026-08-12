import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { LayoutGrid, List, Plus, Search, SlidersHorizontal, Upload } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { SearchFilters, ViewMode } from './files'

interface FileToolbarProps {
  viewMode: ViewMode
  onToggleView: (mode: ViewMode) => void
  searchValue: string
  onSearchChange: (value: string) => void
  onSearch: () => void
  onNewFolder: () => void
  onUpload: (e: React.ChangeEvent<HTMLInputElement>) => void
  uploadPending: boolean
  filters: SearchFilters
  onFiltersChange: (filters: SearchFilters) => void
}

const KIND_OPTIONS: { value: string; labelKey: string }[] = [
  { value: '', labelKey: 'files.filterAll' },
  { value: 'image/', labelKey: 'files.filterImages' },
  { value: 'video/', labelKey: 'files.filterVideos' },
  { value: 'audio/', labelKey: 'files.filterAudio' },
  { value: 'application/', labelKey: 'files.filterDocuments' },
]

const MB = 1024 * 1024

// FileToolbar is the header action cluster of the files page: search box with
// an advanced filter popover (kind/size/date), list/grid view toggle, new
// folder, and upload button.
export function FileToolbar({
  viewMode,
  onToggleView,
  searchValue,
  onSearchChange,
  onSearch,
  onNewFolder,
  onUpload,
  uploadPending,
  filters,
  onFiltersChange,
}: FileToolbarProps) {
  const { t } = useTranslation()

  const setFilter = (patch: Partial<SearchFilters>) => onFiltersChange({ ...filters, ...patch })

  const resetFilters = () => onFiltersChange({})

  const kind = filters.kind ?? ''

  return (
    <div className="flex flex-wrap items-center gap-2">
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
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button
              variant={kind || filters.minSize || filters.maxSize || filters.from || filters.to ? 'secondary' : 'ghost'}
              size="icon"
              className="h-8 w-8"
              aria-label={t('files.filters')}
            />
          }
        >
          <SlidersHorizontal className="h-4 w-4" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-72">
          <div className="space-y-3 p-2">
            <div className="space-y-1.5">
              <Label className="text-xs">{t('files.filterKind')}</Label>
              <div className="flex flex-wrap gap-1">
                {KIND_OPTIONS.map((opt) => (
                  <button
                    key={opt.labelKey}
                    type="button"
                    onClick={() => setFilter({ kind: opt.value || undefined })}
                    className={cn(
                      'rounded-md border px-2 py-1 text-xs transition-colors',
                      kind === opt.value ? 'border-primary bg-primary/10' : 'hover:bg-accent',
                    )}
                  >
                    {t(opt.labelKey)}
                  </button>
                ))}
              </div>
            </div>
            <div className="grid grid-cols-2 gap-2">
              <div className="space-y-1.5">
                <Label className="text-xs">{t('files.filterMinSize')}</Label>
                <Input
                  type="number"
                  min={0}
                  placeholder="MB"
                  value={filters.minSize ? String(Math.round(Number(filters.minSize) / MB)) : ''}
                  onChange={(e) =>
                    setFilter({ minSize: e.target.value ? String(Math.round(Number(e.target.value) * MB)) : undefined })
                  }
                />
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">{t('files.filterMaxSize')}</Label>
                <Input
                  type="number"
                  min={0}
                  placeholder="MB"
                  value={filters.maxSize ? String(Math.round(Number(filters.maxSize) / MB)) : ''}
                  onChange={(e) =>
                    setFilter({ maxSize: e.target.value ? String(Math.round(Number(e.target.value) * MB)) : undefined })
                  }
                />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-2">
              <div className="space-y-1.5">
                <Label className="text-xs">{t('files.filterFrom')}</Label>
                <Input
                  type="date"
                  value={filters.from ?? ''}
                  onChange={(e) => setFilter({ from: e.target.value || undefined })}
                />
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">{t('files.filterTo')}</Label>
                <Input
                  type="date"
                  value={filters.to ?? ''}
                  onChange={(e) => setFilter({ to: e.target.value || undefined })}
                />
              </div>
            </div>
            <div className="flex justify-end">
              <Button variant="ghost" size="sm" onClick={resetFilters}>
                {t('files.filterReset')}
              </Button>
            </div>
          </div>
        </DropdownMenuContent>
      </DropdownMenu>
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
