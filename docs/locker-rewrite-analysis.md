# Locker → Go Rewrite — Analysis Reference

Source: `D:\Github\reference\Locker` (Next.js 16 + tRPC 11 + Drizzle ORM + BetterAuth on PostgreSQL 16).
Target: this repo's Go/Gin + React 19 boilerplate (Authula auth, schema-per-tenant, single binary).

Goal: rewrite Locker in Go so **storage backends are Local drive, Google Drive, and any S3-compatible connector** (S3/R2/MinIO/etc.), with Locker's feature set layered on top.

---

## 1. What Locker Is

Self-hostable Dropbox/Google Drive alternative. Feature surface:

- **File Explorer** — upload, organize, rename, move, delete; grid/list views; previews (PDF, image, MD, CSV, audio, video, text)
- **Tags** — color-coded, workspace-scoped, filterable
- **Share Links** — password, expiry, download limits (token-authenticated public access)
- **Upload Links** — anonymous uploads into a folder, with constraints (max files, max size, allowed MIME, password)
- **Tracked Links** — analytics on views/downloads (visitor, geo, browser, UTM)
- **Multi-Store Storage** — attach Local/S3/R2/Vercel Blob per workspace; primary store for writes; writable replicas sync automatically; read-only ingest scans external buckets
- **Storage Quotas** — per-user and per-workspace byte counters (5 GB defaults)
- **S3-Compatible Gateway** — full virtual bucket API (`/api/s3/*`) with SigV4 auth + XML + multipart
- **Knowledge Base / AI chat / Transcription / Plugin system / VFS shell / browser extension** — advanced features (deferrable)

---

## 2. Data Model Summary (40 tables)

### Tenant isolation
Locker uses **row-level multi-tenancy**: every tenant table carries `workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE`, indexed. No schemas, no RLS.
The rewrite keeps the boilerplate's **schema-per-tenant** (`tenant_<slug>`), so `workspace_id` becomes implicit per tenant schema. All Locker tables go into the tenant schema.

### Core tables
| Table | Key columns / notes |
|---|---|
| `users` | id (text PK, BetterAuth), email unique, `storage_used`, `storage_limit` (5 GB) |
| `workspaces` | id (uuid PK), name, slug (globally unique), owner_id, `storage_used/limit`, theme_config jsonb |
| `workspace_members` | workspace_id, user_id, role (`owner\|admin\|member`), unique (workspace_id, user_id) |
| `workspace_invites` | email, role, token (unique), status (`pending\|accepted\|expired`), expires_at |
| `folders` | workspace_id, user_id, parent_id (self-ref), name, color |
| `files` | workspace_id, user_id, folder_id (set null), **blob_id → file_blobs** (cascade), name, mime_type, size, storage_path, status (`uploading\|ready\|failed`), checksum, s3_key, replaces_file_id; partial unique (workspace_id, folder_id, name) WHERE status='ready' |
| `tags` / `file_tags` | workspace-scoped; slug unique per workspace; M2M join |
| `share_links` | token (unique), access (`download\|raw`), password_hash, expires_at, max_downloads, download_count, file_id XOR folder_id |
| `upload_links` | token (unique), folder_id, name, max_files, max_file_size, allowed_mime_types jsonb, password_hash, expires_at, files_uploaded |
| `tracked_links` | token (unique, 16 bytes), file/folder target, access (`view\|download`), password, require_email, valid_from/until, max_views, view_count, download_count |
| `tracked_link_events` | event_type (`view\|download`), visitor_id, geo fields, UA/browser/OS/device, referrer, UTM, duration_seconds |
| `s3_api_keys` | access_key_id (unique), encrypted_secret (AES-256-GCM), permissions (`readonly\|readwrite`) |
| `s3_multipart_uploads` / `s3_multipart_parts` | upload_id, s3_key, storage_path, content_type, status; parts: part_number, size, etag |
| `stores` | workspace_id, name, **provider enum (`s3\|r2\|vercel_blob\|local` — we add `gdrive`)**, credential_source (`platform\|store`), status (`active\|disabled\|archived`), write_mode (`write\|read_only`), ingest_mode (`none\|scan`), read_priority, config jsonb (bucket/region/endpoint/accountId/publicUrl/baseDir/rootPrefix), last_tested_at, last_synced_at |
| `store_secrets` | 1:1 with stores, encrypted_credentials (AES-256-GCM JSON) |
| `workspace_storage_settings` | 1:1 with workspace, primary_store_id |
| `file_blobs` | workspace_id, created_by_id, object_key (unique per workspace = display path), byte_size, mime_type, checksum, state (`pending\|ready\|failed\|deleted`) |
| `blob_locations` | blob_id, store_id, storage_path, state (`pending\|available\|failed\|missing`), origin (`primary_upload\|replicated\|ingested\|manual_import`), last_verified_at, last_error; unique (blob_id, store_id) |
| `replication_runs` / `replication_run_items` | kind (`upload_fanout\|manual_sync\|repair\|ingest\|rebalance\|manual_pull\|manual_push`), status; items: run_id, blob_id, source/target store, status, attempt_count |
| `ingest_tombstones` | store_id, external_path (unique per store), reason |
| `workspace_storage_configs` | **LEGACY** pre-stores single-config table — skip in rewrite |
| `knowledge_bases` / `kb_tags` / `kb_conversations` / `kb_messages` / `kb_file_ingestions` | AI wiki; messages use nanoid text PK, parts jsonb (AI SDK format) |
| `notifications` | user-level, type, title, body, action_url, read |
| `assistant_conversations` / `assistant_messages` | AI assistant chat |
| plugin tables | `plugin_registry_entries`, `workspace_plugins`, `workspace_plugin_secrets`, `plugin_invocation_logs` |
| `file_transcriptions` | file_id, plugin_slug, content, status |

### Quota
`users.storage_used/limit` and `workspaces.storage_used/limit` — bigint counters maintained by the application (no DB trigger). Truth lives in `file_blobs.byte_size`.

---

## 3. Storage Layer Summary

### Provider interface (`packages/storage/src/interface.ts`)
```ts
upload({path, data, contentType, metadata?}) → {url, path}
download(path) → {data: ReadableStream, contentType, size}
getSignedUrl(path, expiresIn?) → string
getUploadUrl({path, contentType, expiresIn?}) → {url, fields?}
delete(path) → void
exists(path) → boolean
list?(prefix) → [{path, size, lastModified}]        // optional
supportsPresignedUpload: boolean
createPresignedUpload? / createMultipartUpload? / getMultipartPartUrls? / completeMultipartUpload? / abortMultipartUpload?   // S3 only
```

### Providers
| Provider | Notes |
|---|---|
| Local | Filesystem; path-traversal guard (`resolvePath`); signed URLs via HMAC-SHA256 over `path:expiry` → base64url (`local-signing.ts`); recursive `list` |
| S3 | AWS SDK v3; custom `endpoint` (this is what makes any S3-compatible connector work: MinIO, R2, Backblaze, Wasabi, GCS compat); presigned + multipart |
| R2 | S3 adapter with accountId-derived endpoint `https://<accountId>.r2.cloudflarestorage.com` + publicUrl |
| Vercel Blob | Token-based; skip in rewrite (not a user goal) |
| **Google Drive** | Exists only as a **plugin** (`apps/web/server/plugins/handlers/google-drive.ts`) for import/export, not a storage provider. Drive API has **no signed URLs and no list-by-prefix** — the Go `Storage` interface must make `GetSignedURL`/`List` optional or graceful (Drive uses `webContentLink`, `Files.List`). **Multi-account pool**: one OAuth app, one `stores` row per Drive account (own refresh token + root folder in `store_secrets`) → N accounts become one quota-aware storage pool |

### Object key convention
- New format: `objectKey` = human-readable display path (`docs/reports/q1.pdf`)
- Platform stores (`credential_source=platform`): storage path = `rootPrefix/workspaceId/displayPath`
- User stores (`credential_source=store`): storage path = `rootPrefix/displayPath`
- Legacy format `workspaceId/blobId/filename` exists in old data; new uploads use display paths

### Upload/serve pipeline
1. `createPendingFileUpload` → inserts `file_blobs` (state `pending`) + `files` (status `uploading`)
2. Stream body to primary store via `storage.upload(storagePath)`
3. `markFileUploadReady` → file status `ready`, blob state `ready`, update quota counters, insert `blob_locations` (origin `primary_upload`)
4. `runFileReadyHooks` → fire-and-forget plugin hooks
5. `syncFileToStores` → fanout to writable replicas
6. Serving: signed URLs (local HMAC or provider presigned), range-request support, content-type from DB

### Sync / ingest
- `syncFileToStores(fileId, resolveSource, sourceStoreId?)` — download from source, upload to every active writable store ≠ source, upsert `blob_locations` (origin `replicated`), track `replication_run_items`
- `syncWorkspaceStores` — all ready files, concurrency 3
- Conflict strategies: `skip | keep_newer | overwrite`
- `ingestFromReadOnlyStore` — `storage.list(rootPrefix)`, skip existing locations + tombstones, create pending upload, copy into primary, record location (origin `ingested`), then fanout
- `pullFromStore` — same with conflict strategy + `replication_runs` tracking; skips `.locker-store-test-` markers

---

## 4. API Surface Summary

### Transport
- tRPC over `/api/trpc` (superjson) with 4 procedure classes: `public`, `protected`, `workspace` (membership), `workspaceAdmin` (owner/admin)
- Workspace context from `x-workspace-slug` header → resolves membership server-side (boilerplate equivalent: `X-Org-Slug` + tenant middleware)
- BetterAuth: email/password + optional Google OAuth; cookie session (7d, sliding 1d)

### Routers → REST mapping (22 routers)
| Locker router | REST path (proposed) | Key operations |
|---|---|---|
| workspaces / members / users | `/api/t/workspace`, `/api/t/members` (boilerplate orgs cover much of this) | list/get/create/update/delete, transfer, invites, roles |
| files | `/api/t/files` | list (paged, filtered, tagSlugs AND, fileTypes, search), search, getDownloadUrl, rename, move, delete-everywhere |
| folders | `/api/t/folders` | list, get, breadcrumbs, create, rename, move (cycle guard), recursive delete |
| uploads | `/api/t/uploads` + `POST /api/upload` + `PUT /api/upload/stream` + `POST /api/upload/public` | getProvider, checkConflicts, initiate (multipart >10MB, 10MB parts, 4-way), complete, abort |
| storage | `/api/t/storage/usage` | used, limit, fileCount, folderCount, percentage |
| tags | `/api/t/tags` | CRUD + setFileTags + getForFiles |
| shares | `/api/t/share-links` + public `/api/shared/:token` | CRUD, access (file/folder), browseFolder, getDownloadUrl (public, increments count) |
| upload-links | `/api/t/upload-links` + public `/api/upload/:token` | CRUD, get, verifyPassword, anonymous upload |
| tracked-links | `/api/t/tracked-links` + public `/api/track/:token` + `POST /api/track` | CRUD, events, analytics aggregates, access/download (records events) |
| stores | `/api/t/stores` | list (isPrimary), create (tests connection, first becomes primary), update, setPrimary, delete, saveCredentials, test, syncStatus, triggerSync |
| storage-config (legacy) | skip | — |
| s3-keys | `/api/t/s3-keys` | list, create (secret shown once), revoke |
| s3 gateway | `/api/s3/*path` | SigV4 auth; GET (Get/Head/ListObjectsV2/ListParts), PUT (PutObject/UploadPart), POST (Create/Complete multipart), DELETE (Delete/Abort); S3-style XML errors |
| knowledge-bases | `/api/t/kb` + `POST /api/kb/chat` | CRUD, sources, wikiPages, wikiGraph, lint, ingestFile, ingestAll, conversations, chat (streams) |
| transcriptions | `/api/t/transcriptions` + `POST /api/transcribe` | getForFile(s), list, transcribe, retryFailed |
| plugins | `/api/t/plugins` | catalog, installed, registerCustom, install, updateConfig, setStatus, uninstall, fileActions, runAction |
| assistant | `/api/t/assistant` + `POST /api/ai/chat` + `POST /api/ai/generate-file` | conversations, chat (streams), generate-file |
| notifications | `/api/notifications` | list (cursor), unreadCount, markRead, markAllRead |
| vfs-shell | `/api/t/vfs-shell` | run command over virtual FS snapshot |

### Public file serve
`GET /api/files/serve/*path?exp=&sig=` — HMAC-verified signed local file stream with range support.

---

## 5. Boilerplate Inventory (what we build on)

| Capability | Location | Notes |
|---|---|---|
| Gin REST server, `{data,error,message}` envelope | `server/internal/handler/response.go` | reuse |
| Authula auth (email/password, cookie+Bearer hybrid) | `server/internal/middleware/auth.go`, config | reuse as-is |
| Schema-per-tenant isolation | `server/internal/middleware/tenant.go`, `migrate/tenant.go` | extend DDL; keep search_path flow |
| Org = tenant root; members with roles | `model/organization*.go`, `handler/org.go` | Locker workspace ↔ org mapping |
| Google Drive OAuth user-flow | `internal/drive/drive.go`, `handler/settings.go`, `cmd/gdrive-auth` | currently global single folder; refactor to per-store instances |
| Global key-value settings | `model/setting.go` (`app_settings`) | reuse for platform-level config |
| Embedded SPA (single binary) | `cmd/server/static/`, `handler/static.go` | reuse |
| React 19 + Vite + Tailwind v4 + shadcn/ui | `web/` | rebuild pages on this |

---

## 6. Deferred (out of v1 scope, keep in mind)

Knowledge base, AI chat, file generation, transcription, plugin system, VFS shell, browser extension, tracked-link geo enrichment (keep basic counts), Vercel Blob provider, legacy `workspace_storage_configs` table.
