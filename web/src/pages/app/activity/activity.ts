import type { ComponentType } from 'react'
import { Download, History, Move, Plus, Trash2 } from 'lucide-react'

export interface AuditLog {
  id: string
  action: string
  entity_type: string
  entity_id: string | null
  metadata: Record<string, unknown> | string | null
  created_at: string
}

export interface AuditLogsData {
  logs: AuditLog[]
}

export interface BadgeStyle {
  className: string
  Icon: ComponentType<{ className?: string }>
}

// badgeForAction maps an audit action to a colored icon chip by substring:
// CREATE/UPLOAD → emerald/Plus, DELETE/PERMANENT/TRASH → rose/Trash2,
// MOVE → indigo/Move, DOWNLOAD → blue/Download, else slate/History.
export function badgeForAction(action: string): BadgeStyle {
  const upper = action.toUpperCase()
  if (upper.includes('CREATE') || upper.includes('UPLOAD')) {
    return { className: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400', Icon: Plus }
  }
  if (upper.includes('DELETE') || upper.includes('PERMANENT') || upper.includes('TRASH')) {
    return { className: 'bg-rose-500/10 text-rose-600 dark:text-rose-400', Icon: Trash2 }
  }
  if (upper.includes('MOVE')) {
    return { className: 'bg-indigo-500/10 text-indigo-600 dark:text-indigo-400', Icon: Move }
  }
  if (upper.includes('DOWNLOAD')) {
    return { className: 'bg-blue-500/10 text-blue-600 dark:text-blue-400', Icon: Download }
  }
  return { className: 'bg-muted text-muted-foreground', Icon: History }
}

// actionLabelKey maps an audit action to its activity.* i18n label.
export function actionLabelKey(action: string): string {
  const labels: Record<string, string> = {
    file_upload: 'activity.action.fileUpload',
    file_delete: 'activity.action.fileDelete',
    file_delete_batch: 'activity.action.fileDeleteBatch',
    file_move: 'activity.action.fileMove',
    file_move_batch: 'activity.action.fileMoveBatch',
    folder_create: 'activity.action.folderCreate',
    folder_delete: 'activity.action.folderDelete',
    api_key_create: 'activity.action.apiKeyCreate',
    api_key_revoke: 'activity.action.apiKeyRevoke',
  }
  return labels[action] ?? 'activity.action.unknown'
}

export interface MetadataPart {
  kind: 'name' | 'count' | 'size' | 'json'
  value: string
}

// metadataParts normalizes a jsonb column that may arrive as an object or a
// JSON string, extracting the fields the activity row renders.
export function metadataParts(metadata: AuditLog['metadata']): MetadataPart[] {
  if (!metadata) return []
  let obj: Record<string, unknown>
  if (typeof metadata === 'string') {
    try {
      obj = JSON.parse(metadata) as Record<string, unknown>
    } catch {
      return [{ kind: 'json', value: metadata }]
    }
  } else {
    obj = metadata
  }
  const parts: MetadataPart[] = []
  for (const key of ['name', 'fileName', 'folderName']) {
    const value = obj[key]
    if (typeof value === 'string' && value) parts.push({ kind: 'name', value })
  }
  if (typeof obj.count === 'number') parts.push({ kind: 'count', value: String(obj.count) })
  if (typeof obj.size === 'number') parts.push({ kind: 'size', value: formatBytes(obj.size) })
  if (parts.length === 0) parts.push({ kind: 'json', value: JSON.stringify(obj) })
  return parts
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / k ** i).toFixed(1))} ${sizes[i]}`
}
