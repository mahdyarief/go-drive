import { Cloud, Database, HardDrive } from 'lucide-react'
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

// copyText copies to the clipboard and resolves true on success so callers can
// show feedback when the browser blocks the write (permissions, non-secure
// context).
export async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    return false
  }
}

// bytesToGB converts a byte count to gigabytes for the quota input field.
export function bytesToGB(bytes: number): number {
  if (!Number.isFinite(bytes) || bytes <= 0) return 0
  return bytes / 1024 ** 3
}

// localStorage key used to notify other tabs when a Google Drive connect
// completes in the OAuth tab (the `storage` event fires in every other tab).
export const GDRIVE_CONNECTED_KEY = 'gdrive:connected'

// providerLabel returns the translated name for a store provider. Shared by
// StoreCard (per-card badge) and StoreGroups (section headers) so they can
// never drift apart.
export function providerLabel(t: (key: string) => string, provider: string): string {
  if (provider === 'local') return t('stores.providerLocal')
  if (provider === 's3') return t('stores.providerS3')
  if (provider === 'gdrive') return t('stores.providerGdrive')
  if (provider === 'b2') return t('stores.providerB2')
  if (provider === 'wasabi') return t('stores.providerWasabi')
  if (provider === 'spaces') return t('stores.providerSpaces')
  if (provider === 'hetzner') return t('stores.providerHetzner')
  if (provider === 'idrivee2') return t('stores.providerIdrivee2')
  if (provider === 'storj') return t('stores.providerStorj')
  return provider
}

// PROVIDER_ICONS maps each provider to its lucide icon. Shared by StoreCard
// (row icon) and StoreGroups (section header) so they can never drift apart.
export const PROVIDER_ICONS = {
  gdrive: Cloud,
  s3: Database,
  b2: Database,
  wasabi: Database,
  spaces: Database,
  hetzner: Database,
  idrivee2: Database,
  storj: Database,
  local: HardDrive,
} as const

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

export type Provider = 'local' | 's3' | 'gdrive' | 'b2' | 'wasabi' | 'spaces' | 'hetzner' | 'idrivee2' | 'storj'

export type WriteMode = 'write' | 'writeonly' | 'none'

export type IngestMode = 'none' | 'poll' | 'webhook'

export interface StoreForm {
  name: string
  provider: Provider
  writeMode: WriteMode
  ingestMode: IngestMode
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
