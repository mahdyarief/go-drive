// ApiResponse is the standard response envelope from all API endpoints.
// Mirrors the Go handler.ApiResponse struct.
export interface ApiResponse<T = unknown> {
  data?: T
  error?: string
  message?: string
}

// User from the auth system.
export interface User {
  id: string
  name: string
  email: string
  is_admin?: boolean
}

// Organization with the user's role in it.
export interface Organization {
  id: string
  name: string
  slug: string
  role: 'owner' | 'admin' | 'member'
}

// Response shapes for specific endpoints.

export interface MeData {
  user: User
  organizations: Organization[]
}

export interface OrgData {
  organization: Organization
}

export interface OrgListData {
  organizations: Organization[]
}

export interface OrgDetailsData {
  organization: {
    id: string
    name: string
    slug: string
    created_at: string
  }
  members: {
    id: string
    user_id: string
    role: string
  }[]
  your_role: string
}

export interface TenantStatusData {
  org_id: string
  org_slug: string
  schema: string
  role: string
}

export interface AdminOrg {
  id: string
  name: string
  slug: string
  created_at?: string
  member_count?: number
}

export interface AdminOrgMember {
  id: string
  user_id: string
  name: string
  email: string
  role: string
  created_at: string
}

export interface AdminOrgDetail {
  org: AdminOrg
  members: AdminOrgMember[]
}

export interface RegisterSettings {
  register_disabled: boolean
}

export interface AdminUser {
  id: string
  name: string
  email: string
  created_at: string
  is_admin: boolean
  org_count: number
}

export interface GDriveSettings {
  configured: boolean
  connected: boolean
  folder_id: string
  client_id_masked: string
  redirect_uri: string
}

export interface GDriveStorageQuota {
  limit: number
  usage: number
  usage_in_drive: number
  usage_in_drive_trash: number
}

// --- Locker (M9) models ---

export interface LockerFile {
  id: string
  user_id: string
  folder_id: string | null
  blob_id: string
  name: string
  mime_type: string
  size: number
  storage_path: string
  storage_provider: string
  status: string
  thumbnail_path: string
  checksum: string
  s3_key: string
  replaces_file_id: string | null
  created_at: string
  updated_at: string
}

// FileStoreInfo is the store that holds a file's blob (from blob_locations).
export interface FileStoreInfo {
  id: string
  name: string
  provider: string
}

export interface Folder {
  id: string
  user_id: string
  parent_id: string | null
  name: string
  color: string
  created_at: string
  updated_at: string
}

export interface BreadcrumbItem {
  id: string
  name: string
}

export interface StorageUsage {
  used: number
  limit: number
  fileCount: number
  folderCount: number
  percentage: number
}

export interface Tag {
  id: string
  name: string
  slug: string
  color: string
  created_at: string
  updated_at: string
}

export interface Store {
  id: string
  name: string
  provider: string
  credential_source: string
  status: string
  write_mode: string
  ingest_mode: string
  read_priority: number
  config: Record<string, unknown>
  quota_used: number
  quota_limit: number
  last_tested_at: string | null
  last_synced_at: string | null
  created_at: string
  updated_at: string
}

export interface S3Key {
  id: string
  user_id: string
  access_key_id: string
  encrypted_secret: string
  name: string
  permissions: string
  is_active: boolean
  last_used_at: string | null
  expires_at: string | null
  created_at: string
}

export interface ShareLink {
  id: string
  user_id: string
  file_id: string | null
  folder_id: string | null
  token: string
  access: string
  has_password: boolean
  password_hash: string
  expires_at: string | null
  max_downloads: number | null
  download_count: number
  is_active: boolean
  last_accessed_at: string | null
  created_at: string
  updated_at: string
}

export interface UploadLink {
  id: string
  user_id: string
  folder_id: string | null
  token: string
  name: string
  max_files: number | null
  max_file_size: number | null
  allowed_mime_types: string[]
  files_uploaded: number
  has_password: boolean
  password_hash: string
  expires_at: string | null
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface TrackedLink {
  id: string
  user_id: string
  file_id: string | null
  folder_id: string | null
  token: string
  name: string
  description: string
  access: string
  has_password: boolean
  password_hash: string
  require_email: boolean
  expires_at: string | null
  valid_from: string | null
  valid_until: string | null
  max_views: number | null
  view_count: number
  download_count: number
  is_active: boolean
  last_accessed_at: string | null
  created_at: string
  updated_at: string
}

export interface TrackedLinkEvent {
  id: string
  tracked_link_id: string
  event_type: string
  timestamp: string
  visitor_id: string
  email: string
  ip_address: string
  country: string
  country_code: string
  region: string
  city: string
  latitude: number | null
  longitude: number | null
  user_agent: string
  browser: string
  browser_version: string
  os: string
  os_version: string
  device_type: string
  referrer: string
  utm_source: string
  utm_medium: string
  utm_campaign: string
  language: string
  duration_seconds: number | null
}

export interface ReplicationRun {
  id: string
  kind: string
  status: string
  source_store_id: string | null
  target_store_id: string | null
  triggered_by_user_id: string
  total_items: number
  processed_items: number
  failed_items: number
  error_message: string
  started_at: string | null
  completed_at: string | null
  created_at: string
  updated_at: string
}
