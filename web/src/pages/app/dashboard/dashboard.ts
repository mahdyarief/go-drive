import type { LockerFile } from '@/lib/types'

export interface RecentFilesData {
  files: LockerFile[]
}

export interface RecentFoldersData {
  folders: {
    id: string
    name: string
    color: string
  }[]
}

export const RECENT_FOLDERS_LIMIT = 4
export const RECENT_FILES_LIMIT = 5
export const RECENT_ACTIVITY_LIMIT = 5
