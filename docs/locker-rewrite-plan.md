# Locker → Go Rewrite — Modular Implementation Plan

Implements the analysis in `docs/locker-rewrite-analysis.md` on the go-drive boilerplate.
Primary goal: storage backends = **Local drive + Google Drive + S3-compatible connectors**.

---

## Architecture Decisions (recap)

1. **Tenant mapping**: Locker workspace → boilerplate org (tenant schema). `X-Org-Slug` ≡ `x-workspace-slug`. Roles owner/admin/member map to org member roles.
2. **Schema-per-tenant**: all Locker tables live in `tenant_<slug>`; `workspace_id` is implicit (org scope), no row-level `workspace_id` columns needed.
3. **Storage interface** (port of `StorageProvider`):
   ```go
   type Object struct {
       Path         string
       Size         int64
       LastModified time.Time
   }

   type Storage interface {
       Upload(ctx context.Context, path string, r io.Reader, contentType string) error
       Download(ctx context.Context, path string) (io.ReadCloser, int64, error) // body, size, err
       GetSignedURL(ctx context.Context, path string, expiresIn time.Duration) (string, error)
       Delete(ctx context.Context, path string) error
       Exists(ctx context.Context, path string) (bool, error)
       List(ctx context.Context, prefix string) ([]Object, error) // optional
   }
   ```
   Google Drive implements `GetSignedURL` gracefully (returns `webContentLink`-style link or an error surfaced as "not supported") and `List` via `Files.List` query.
4. **Provider registry** = Locker's `createStorageFromConfig`: `storage.New(config)` switches on provider (local|s3|gdrive), reads encrypted credentials from `store_secrets`.
5. **Async jobs**: replication fanout + ingest run as goroutines with semaphore concurrency (3), tracked in `replication_runs` / `replication_run_items`.
6. **Storage pool (multi-account Drive / multi-store)**: a workspace's stores act as one pool usable like S3. One OAuth app, N Drive accounts (each = one `stores` row with its own refresh token + root folder in `store_secrets`). Reads use `blob_locations` (exact) with `read_priority` fallback; writes use either quota-aware placement (pick the store with most free space) or fanout replication. The S3 gateway makes the pool invisible to S3 clients.

### Pool scheduling rules
- `workspace_storage_settings.primary_store_id` → extended to a **pool** (ordered list of write-capable stores) for quota-aware placement; keep single-primary mode as the default (fanout to replicas only).
- Placement: query each pool member's quota (`About.storageQuota` for Drive, `HeadBucket`/`ListObjects` heuristics for S3, filesystem stat for Local), pick max free space; tie-break by `read_priority`, then `created_at`.
- Read: exact lookup via `blob_locations` (blob → store → storage_path). On store failure, try next location by `read_priority`.
- Failure handling: failed placement → retry next store; if all fail, mark `blob_locations` state `failed` + `last_error`, surface as `StorageUnavailable` (S3 error XML variant).

---

## Module M1 — Database layer

**Files**: `server/internal/model/*.go` (new Bun models), `server/internal/migrate/tenant.go` (DDL).

New Bun models (tenant schema):
`File`, `Folder`, `FileBlob`, `BlobLocation`, `Store`, `StoreSecret`, `WorkspaceStorageSetting`, `Tag`, `FileTag`, `ShareLink`, `UploadLink`, `TrackedLink`, `TrackedLinkEvent`, `S3APIKey`, `S3MultipartUpload`, `S3MultipartPart`, `ReplicationRun`, `ReplicationRunItem`, `IngestTombstone`, `Notification`, `FileTranscription`.

DDL in `CreateTenantSchema()`:
- `CREATE TYPE store_provider AS ENUM ('local','s3','r2','gdrive')` (+ existing r2 kept for compat; vercel_blob omitted)
- `CREATE TYPE blob_state`, `blob_location_state`, `blob_location_origin`, `replication_run_kind`, `replication_run_status`, `replication_run_item_status`, `ingest_tombstone_reason`, `tracked_link_event_type`
- All tables from the analysis doc §2, with `workspace_id` **omitted** (implicit via schema) — except `notifications` (user-level → public schema, keyed by Authula user id) and `file_transcriptions` (kept workspace-scoped).

**Accept**: `make dev-server` boots with migrations applying cleanly.

---

## Module M2 — Crypto (secrets at rest)

**Files**: `server/internal/crypto/crypto.go`.

- AES-256-GCM encrypt/decrypt (port of `apps/web/server/stores/crypto.ts`)
- Key from env `SECRETS_ENCRYPTION_KEY` (fallback: derive from `AUTHULA_SECRET`)
- Exports: `Encrypt(plaintext string) (string, error)`, `Decrypt(ciphertext string) (string, error)`

Used by: `store_secrets.encrypted_credentials`, `s3_api_keys.encrypted_secret`, plugin secrets.

---

## Module M3 — Storage providers

**Files**: `server/internal/storage/interface.go`, `local.go`, `s3.go`, `gdrive.go`, `factory.go`.

1. **Local** — `baseDir` from config (`LOCAL_BLOB_DIR` or per-store `baseDir`); path-traversal guard; HMAC-SHA256 signed URLs (`path:expiry` → base64url) serving `/api/files/serve/*`; recursive `List`.
2. **S3-compatible** — AWS SDK v2 (`aws-sdk-go-v2/service/s3`); config: `accessKeyId`, `secretAccessKey`, `bucket`, `region`, `endpoint` (custom endpoint = MinIO/R2/Backblaze/etc.), `publicUrl`. Presigned URLs (GetObject/PutObject), multipart (Create/UploadPart/Complete/Abort) with 10 MB parts, 4-way concurrency.
3. **Google Drive** — refactor `server/internal/drive/drive.go`: add constructor `drive.NewStore(clientID, clientSecret, refreshToken, folderID) *Store` (per-store instance, no package-global state); keep existing package-level API for the legacy admin settings flow. Methods: `Upload` (`Files.Create.Media`), `Download` (`Files.Get.Download`), `Delete` (`Files.Delete`), `Exists` (`Files.Get` fields=id), `List` (`Files.List` `'folderID' in parents`), `GetSignedURL` → returns `webContentLink` (or error if permission not `anyone`-readable), `Quota` (`About.storageQuota` — used by pool placement). Add `drive.gdrive` provider in `factory.go`. Multi-account = multiple `stores` rows sharing one OAuth app.
4. **Factory** — `New(cfg Config) (Storage, error)` switch on provider; decrypts credentials via M2.

**Accept**: unit tests for Local (round-trip, traversal guard, signature verify), S3 (against MinIO in docker-compose), Drive (against test folder, optional).

---

## Module M4 — Upload / serve pipeline

**Files**: `server/internal/handler/upload.go`, `file.go`, `serve.go`; `server/internal/store/file_records.go`.

Port of `createPendingFileUpload` / `markFileUploadReady` / `deleteFileEverywhere`:
- `POST /api/upload` — multipart (fields `file`, `folderId?`, `fileId?` replace), authed + tenant; validate `MAX_FILE_SIZE` (100 MB), quota (`users.storage_used/limit`, `workspaces.storage_used/limit`); create `file_blobs` (pending) + `files` (uploading); stream to **primary store**; mark ready; update quota counters; insert `blob_locations` (origin `primary_upload`); fire async sync fanout (M7).
- `PUT /api/upload/stream` — raw streaming body (metadata via `x-file-id`, `x-workspace-slug` headers) for browser extension.
- `GET /api/files/serve/*path?exp=&sig=` — HMAC-verified (M3-local); `http.ServeContent` for range requests; Content-Type from DB.
- `GET /api/t/files/:id/download-url` — returns signed URL (3600s TTL) via provider `GetSignedURL`.

**Accept**: upload → file appears in DB with status `ready`; file streams back with Range support; quota increments.

---

## Module M5 — File explorer REST

**Files**: `server/internal/handler/file.go`, `folder.go`, `tag.go`; routes in `router.go` tenant group.

- Folders: `GET/POST /api/t/folders`, `GET /api/t/folders/breadcrumbs?folderId=`, `PATCH /api/t/folders/:id` (rename/move w/ cycle guard), `DELETE /api/t/folders/:id` (recursive, files → root).
- Files: `GET /api/t/files?folderId=&search=&tagSlugs=&fileTypes=&page=&pageSize=&sort=` (dot-folder `.plugins` excluded, partial-unique name-in-folder enforced), `PATCH /api/t/files/:id` (rename/move, updates objectKey + blob locations), `DELETE /api/t/files/:id` (delete everywhere: remove blob from all stores + `blob_locations` rows), `GET /api/t/files/search?q=` (transcription ILIKE fallback).
- Tags: `GET/POST /api/t/tags`, `PATCH/DELETE /api/t/tags/:id`, `POST /api/t/tags/set-file-tags`, `POST /api/t/tags/for-files`.
- Storage usage: `GET /api/t/storage/usage`.

**Accept**: full explorer CRUD works against any provider (local dev default).

---

## Module M6 — Links (share / upload / tracked)

**Files**: `server/internal/handler/share.go`, `uploadlink.go`, `trackedlink.go`; `server/internal/security/password.go`.

- Password hashing: bcrypt (`hashLinkPassword`/`verifyLinkPassword`).
- Token generation: 32-char (share/upload), 16-byte hex (tracked).
- **Share links**: workspace CRUD (`/api/t/share-links`) + public `GET /api/shared/:token` (file or folder browse, password/expiry/maxDownloads checks, increments `download_count`), `GET /api/shared/:token/download?fileId=` (folder descendants only), raw variant.
- **Upload links**: workspace CRUD + public `POST /api/upload/public` (token + optional password; enforces maxFiles/maxFileSize/allowedMimeTypes/expiry; increments `files_uploaded`).
- **Tracked links**: workspace CRUD + public access endpoint recording `tracked_link_events` (view/download; keep IP/UA/basic counts; geo enrichment deferred).

**Accept**: public link flows work end-to-end against any provider.

---

## Module M7 — Multi-store replication + ingest

**Files**: `server/internal/store/sync.go`, `path_builder.go`, `ingest.go`.

Port of `syncFileToStores` / `syncWorkspaceStores` / `ingestFromReadOnlyStore` / `pullFromStore`:
- Path builder: platform → `rootPrefix/<org>/<displayPath>`, user store → `rootPrefix/<displayPath>`; legacy key detection + dedup (` (1)`, ` (2)`, …).
- `syncFileToStores(fileID, sourceStoreID, conflictStrategy)` — download from source, upload to active writable targets ≠ source, upsert `blob_locations` (origin `replicated`), upsert `replication_run_items`.
- Workspace sync: goroutines + semaphore (3), `replication_runs` progress tracking.
- Ingest: `storage.List(rootPrefix)`, skip existing `blob_locations` + `ingest_tombstones`, copy into primary, record location (origin `ingested`), fanout; skip `.locker-store-test-` markers on pull.
- Store CRUD: `/api/t/stores` (create tests connection first; first store becomes primary via `workspace_storage_settings`), `saveCredentials` (encrypt → `store_secrets`), `test`, `setPrimary`, `syncStatus`, `triggerSync`, delete.

**Accept**: two stores attached → new upload appears on both; read-only ingest pulls external bucket objects.

---

## Module M8 — S3-compatible gateway

**Files**: `server/internal/s3/auth.go` (SigV4), `server/internal/handler/s3gateway.go`; routes at `/api/s3/*path`.

- Auth: AWS SigV4 signature verification against `s3_api_keys` (access_key_id + decrypted secret); `readonly` blocks writes.
- Path: `/{workspaceSlug}/{key}`.
- Handlers: GET (GetObject/HeadObject with range, `?list-type=2` ListObjectsV2 XML, `?uploads` ListMultipartUploads, `?uploadId`+`?partNumber` ListParts), PUT (PutObject, `?uploadId`+`?partNumber` UploadPart), POST (`?uploads` CreateMultipartUpload, `?uploadId` CompleteMultipartUpload), DELETE (DeleteObject → `deleteFileEverywhere`, `?uploadId` AbortMultipartUpload).
- S3-style XML errors (`AccessDenied`, `NoSuchBucket`, `NoSuchKey`, `QuotaExceeded`, `NoSuchUpload`).
- Folder chain auto-create via display path (`resolveOrCreateFolderChain`).

**Accept**: `aws cli s3 --endpoint-url http://localhost:8081/api/s3 ls/cp/rm` works against a workspace.

---

## Module M9 — Frontend (React)

Rebuild on the existing shadcn/ui stack. Pages:
- Auth + workspace switcher (exists)
- File explorer (grid/list, breadcrumbs, upload button w/ progress, right-click menu: rename/move/delete/tag/share)
- File preview/detail page
- Settings: members/invites, appearance, **Stores** (attach Local/Drive/S3 with BYO credentials, set primary, test, sync/ingest buttons, quota display), API keys (S3), notifications
- Links: share-links list, upload-links list, tracked-links list + analytics view
- Public pages: `/shared/:token`, `/upload/:token`, `/tracked/:token`

**Accept**: explorer + stores settings + links pages functional against REST API.

---

## Module M10 — Deferred (explicitly out of v1)

Knowledge base, AI chat/file generation, transcription service, plugin system, VFS shell, browser extension, tracked-link geo enrichment, Vercel Blob provider, legacy `workspace_storage_configs`.

---

## Recommended Execution Order

1. M1 DB → M2 crypto → M3 providers (local first, then S3, then Drive) → M4 upload/serve
2. M5 explorer → M6 links → M9 frontend (explorer + settings/stores)
3. M7 replication → M8 S3 gateway
4. M10 deferred

## Dependencies

| Module | Needs |
|---|---|
| M3 | M1 (store rows), M2 (decrypt credentials) |
| M4 | M1, M3, M7 (sync fanout hook) |
| M5 | M1, M4 |
| M6 | M1, M3 (signed URLs), M4 |
| M7 | M1, M3 |
| M8 | M1, M2 (s3_api_keys), M4/M5 (file ops), M7 |
| M9 | M4, M5, M6, M7 |

## New Go dependencies

- `github.com/aws/aws-sdk-go-v2` + `.../service/s3` + `.../feature/s3/manager`
- `golang.org/x/crypto/bcrypt` (link passwords)
- `golang.org/x/oauth2` + `google.golang.org/api/drive/v3` (already in go.mod)
