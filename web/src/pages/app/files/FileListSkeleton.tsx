import type { ViewMode } from './files'

interface FileListSkeletonProps {
  viewMode: ViewMode
}

const LIST_ROWS = 6
const GRID_TILES = 8

// FileListSkeleton mirrors FileList's layout with pulse placeholders so the
// loading state matches the upcoming content instead of a bare "..." line.
export function FileListSkeleton({ viewMode }: FileListSkeletonProps) {
  if (viewMode === 'grid') {
    return (
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
        {Array.from({ length: GRID_TILES }, (_, i) => (
          <div key={i} className="flex flex-col items-center gap-2 rounded-lg border p-4">
            <div className="h-8 w-8 animate-pulse rounded-full bg-muted" />
            <div className="h-3 w-3/4 animate-pulse rounded bg-muted" />
            <div className="h-2 w-1/2 animate-pulse rounded bg-muted" />
          </div>
        ))}
      </div>
    )
  }

  return (
    <ul className="divide-y divide-border">
      {Array.from({ length: LIST_ROWS }, (_, i) => (
        <li key={i} className="flex items-center gap-3 py-2">
          <div className="h-4 w-4 shrink-0 animate-pulse rounded bg-muted" />
          <div className="h-3 flex-1 animate-pulse rounded bg-muted" />
          <div className="h-3 w-16 shrink-0 animate-pulse rounded bg-muted" />
        </li>
      ))}
    </ul>
  )
}
