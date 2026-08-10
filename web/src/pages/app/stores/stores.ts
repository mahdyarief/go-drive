import { useOrgStore } from '@/store/org'
import type { ReplicationRun, S3Key, Store } from '@/lib/types'

// formatBytes renders a byte count in a human-readable unit (KB/MB/GB/TB).
export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)))
  const value = bytes / 1024 ** i
  return `${value.toFixed(value >= 10 || i === 0 ? 0 : 1)} ${units[i]}`
}

export function copyText(text: string) {
  void navigator.clipboard.writeText(text)
}

// bytesToGB converts a byte count to gigabytes for the quota input field.
export function bytesToGB(bytes: number): number {
  if (!Number.isFinite(bytes) || bytes <= 0) return 0
  return bytes / 1024 ** 3
}

// emptyForm returns a fresh create-store form state.
export function emptyForm(): StoreForm {
  return { name: '', provider: 'local', writeMode: 'write', ingestMode: 'none', readPriority: 100, quotaLimit: 0, config: {}, credentials: {} }
}

// localStorage key used to notify other tabs when a Google Drive connect
// completes in the OAuth tab (the `storage` event fires in every other tab).
export const GDRIVE_CONNECTED_KEY = 'gdrive:connected'

export interface StoresData {
  stores: Store[]
  primaryStoreId: string | null
  storageMode?: string
  gdriveRedirectUri?: string
}

export interface TestStoreData {
  ok: boolean
  used: number
  limit: number
}

export interface IngestData {
  ingested: number
}

export interface SyncData {
  runs: ReplicationRun[]
}

export interface TriggerSyncData {
  run: ReplicationRun
}

export interface KeysData {
  keys: S3Key[]
}

export interface CreateKeyData {
  key: S3Key
  accessKeyId: string
  secretAccessKey: string
}

export type Provider = 'local' | 's3' | 'gdrive'

export interface StoreForm {
  name: string
  provider: Provider
  writeMode: string
  ingestMode: string
  readPriority: number
  quotaLimit: number // GB
  config: Record<string, string>
  credentials: Record<string, string>
}

export interface GDriveCompleteData {
  ok: boolean
  used: number
  limit: number
  storeId: string
}

export function useOrgSlug(): string | undefined {
  return useOrgStore((s) => s.currentOrg?.slug)
}
