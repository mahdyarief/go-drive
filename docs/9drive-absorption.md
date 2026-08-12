# 9Drive Feature Absorption — Roadmap & Progress

> **Source reference**: [`D:\Github\reference\9drive`](D:\Github\reference\9drive) — Express/TS + Prisma/MySQL + React/Vite storage gateway (Google Drive + S3-compatible). Mirip dengan go-drive, sehingga fiturnya relevan untuk diserap.

**Tujuan dokumen**: peta pengembangan (roadmap) dan pelacak progress implementasi fitur 9drive yang diserap ke go-drive. Update dokumen ini setiap kali ada batch fitur yang selesai dikerjakan (setelah commit + push, sebelum tutup sesi).

---

## 1. Gap Analysis: 9drive vs go-drive

Berdasarkan pembacaan README.md, AGENTS.md, `backend/prisma/schema.prisma` (353 baris), dan file-file route kunci (`upload.routes.ts`, `public-api.routes.ts`, `api-key.routes.ts`, `api-key.middleware.ts`, `storage.routes.ts`) per 2026-08-12.

| # | Fitur 9drive | Status di go-drive | Nilai absorpsi |
|---|---|---|---|
| 1 | **Multi-upload batch** — `POST /uploads` menerima `filesMeta` (JSON array `{fieldName, fileName, mimeType, sizeBytes, folderId}`) lalu field file `file-0`, `file-1`, dst; response `{files, failed}` dengan per-file status | go-drive hanya single upload (`POST /api/t/upload`) | **Tinggi** |
| 2 | **Upload routing policy** — `UploadRoutingPolicy` per-user: mode `most_available` \| `round_robin` \| `priority` + `priorityAccountIds` + `roundRobinCursor`; auto-sync quota stale (>5 menit) sebelum memilih; `reservedBytesByAccount` untuk multi-file dalam satu request | go-drive hanya `store.ResolvePrimaryStore` (store primer statis) | **Tinggi** |
| 3 | **External upload API + API key** — `POST /api/v1/uploads` + `requireApiKey(scope)` middleware; API key `9d_live_<random>` disimpan sebagai hash (`keyHash`), `keyPrefix` untuk display, scopes (`files:upload`), `lastUsedAt`, `expiresAt`, `revokedAt`; one-time secret display saat create | go-drive punya `s3_api_keys` untuk S3 SigV4 gateway, tapi bukan REST upload API; ada `PublicUpload` tanpa key management | **Tinggi** |
| 4 | **Storage breakdown** — `GET /storage/breakdown`: GROUP BY mime_type → photo/video/document (COALESCE SUM size_bytes) | go-drive hanya `GET /storage/usage` (total) | **Rendah** |
| 5 | **Batch file ops** — `PATCH /files/batch`, `DELETE /files/batch` | go-drive per-file only | **Sedang** |
| 6 | **Audit log** — model `AuditLog` (userId, action, entityType, entityId, metadata JSON) + `createAuditLog()` util + halaman Activity Log | Tidak ada | **Sedang** |
| 7 | **Preview token** — `POST /files/:id/preview-token` → `GET /files/preview/:token` (short-lived, hashed, tanpa auth) | go-drive pakai fetch-by-ID dengan auth (commit d022aba) | **Sedang** |
| 8 | **Recent folders** — `GET /folders/recent?limit=4` | Tidak ada | **Rendah** |
| 9 | **Resumable upload** — `POST /uploads/resumable/init`, `GET .../status/:id`, `PUT .../chunk/:id` (Content-Range, proxy ke Google Drive resumable session) | Tidak ada | **Rendah** |
| 10 | **In-app API docs** — halaman dokumentasi API dengan contoh cURL + JavaScript | Tidak ada | **Sedang** |

---

## 2. Roadmap (urutan prioritas)

### P0 — Inti UX (dikerjakan lebih dulu)
- [x] **#1 Multi-upload batch + progress panel**
  - Backend: upgrade `POST /api/t/upload` (server/internal/handler/upload.go) terima multi-file. Di Go, parse multipart berurutan; kumpulkan `filesMeta` dari field form, lalu stream tiap file ke store. Response `{ files, failed }` per-file status.
  - Frontend: pola `UploadContext` (seperti 9drive `frontend/src/context/UploadContext.tsx`) + panel progress bottom-right; go-drive sudah punya `UploadProgressCard` (refactor c7e22cf) — tinggal dijadikan panel multi-entry.
  - i18n keys baru di `web/src/locales/en.json` / `id.json`.

### P1 — Infrastruktur storage
- [x] **#2 Upload routing policy**
  - Backend: tabel tenant `store_routing_policy` (mode, priority_store_ids, round_robin_cursor) — tambah CREATE TABLE di `server/internal/migrate/tenant.go` + model di `server/internal/model/`.
  - Handler: `GET/PATCH /api/t/storage/routing-policy`; ubah alur pilih store dari `ResolvePrimaryStore` jadi selector policy-aware (mode most_available / round_robin / priority).
  - Auto-refresh quota stale (>5 menit) + reserved-bytes per batch (lihat `selectAccount()` di 9drive `upload.routes.ts:46`).
- [x] **#3 External upload API + API key**
  - Backend: ekstend `s3_api_keys` jadi `api_keys` generik (name, keyPrefix, keyHash, scopes, status, lastUsedAt, expiresAt, revokedAt) atau tabel baru; middleware Bearer-hash lookup seperti 9drive `api-key.middleware.ts`.
  - Route baru: `POST /api/v1/uploads` (public, tanpa session — autentikasi via API key), plus CRUD `GET/POST/DELETE /api/t/api-keys`.
  - One-time secret display saat create; hash disimpan di DB.

### P2 — Penyempurnaan
- [x] **#4 Storage breakdown** — endpoint `GET /api/t/storage/breakdown` (photo/video/document) + tampilan di halaman dashboard/stores.
- [x] **#5 Batch file ops** — `PATCH/DELETE /api/t/files/batch` (body berisi array id) + UI select-multiple di FileList.
- [ ] **#8 Recent folders** — `GET /api/t/folders/recent?limit=4` (ORDER BY updated_at DESC) + quick-nav di sidebar.
- [ ] **#10 In-app API docs** — halaman dokumentasi (cURL + JS examples) untuk upload API.

### P3 — Belakangan (opsional)
- [ ] **#6 Audit log** — tabel `audit_logs` + `createAuditLog` util + halaman Activity Log.
- [ ] **#7 Preview token** — short-lived token untuk preview publik.
- [ ] **#9 Resumable upload** — kompleks, hanya untuk Google Drive; prioritas rendah.

---

## 3. Progress Tracker

| Fitur | Status | Commit | Catatan |
|---|---|---|---|
| #1 Multi-upload + progress | ✅ | `2fdc13d` + `c7e857f` | `POST /api/t/upload` multi-file (`files` field, legacy `file` tetap); response `{files,failed}` per-file; XHR progress panel multi-entry (`lib/upload.ts` + `store/upload.ts` `uploadBatch` + `UploadPanel.tsx`) |
| #2 Upload routing policy | ✅ | `4f2f2cd` (merge `ab93973`) | Tabel tenant `store_routing_policy` (mode most_available/round_robin/priority, priority_store_ids, round_robin_cursor); `GET/PATCH /api/t/storage/routing-policy`; selector policy-aware di `store/file_records.go` (`ResolveUploadStoreReserved` + stale-quota auto-refresh >5 menit); reserved-bytes per batch di `handler/upload.go` |
| #3 External upload API + API key | ✅ | `06cdc4e` (merge `ab93973`) | Tabel `api_keys` public schema (org_slug, key_prefix, key_hash SHA-256, scopes, status, revoked_at); `POST /api/v1/uploads` via `RequireAPIKey` + `APIKeyTenantTx`; CRUD `/api/t/api-keys`; secret `9d_live_` + 40 hex one-time display |
| #4 Storage breakdown | ✅ | `1325d9b` (merge `f0564af`) | `GET /api/t/storage/breakdown` (GROUP BY mime_type → photo/video/document, total); `StorageBreakdownCard.tsx` segmented bar di StoresPage; i18n `storage.*` |
| #5 Batch file ops | ✅ | `2ea0ee5` (merge `f0564af`) + fix `680c7c2` | `PATCH/DELETE /api/t/files/batch` (move/delete by id array, `{moved}`/`{deleted}`); select-multiple mode di FilesPage (`batchOps.ts` + `BatchFileBar.tsx`); fix `moveObject` handle-close Windows + cleanup `blob_locations` di `DeleteFileEverywhere` |
| #6 Audit log | ⬜ Belum dimulai | — | — |
| #7 Preview token | ⬜ Belum dimulai | — | — |
| #8 Recent folders | ✅ | `b82aadb` (merge `e6e73d6`) | `GET /api/t/folders/recent?limit=4` (updated_at DESC, default 4); `RecentFolders.tsx` quick-nav di sidebar; FilesPage baca param `?folder=` |
| #9 Resumable upload | ⬜ Belum dimulai | — | — |
| #10 In-app API docs | ⬜ Belum dimulai | — | — |

**Status legend**: ⬜ Belum dimulai · 🔄 Sedang dikerjakan · ✅ Selesai (committed + pushed)

---

## 4. Catatan teknis dari 9drive (referensi implementasi)

### Upload routing (`upload.routes.ts`)
- `selectAccount(userId, sizeBytes, reservedBytesByAccount, targetAccountId)`:
  1. Ambil semua connected account (google_drive + s3) dengan `storageAccount`.
  2. Filter akun yang quota-nya stale (`lastSyncedAt` > 5 menit) → sync quota paralel (`Promise.allSettled`).
  3. Re-query, hitung `availableBytes = storage.availableBytes - reserved`, filter `availableBytes >= sizeBytes`.
  4. Pilih sesuai policy: `priority` (urut by priorityAccountIds), `round_robin` (cursor index), else `most_available` (sort by availableBytes, s3 preferred kalau keduanya null).
  5. `targetAccountId` override kalau folder tertentu sudah terikat ke akun.
- `reservedBytesByAccount` di-increment per file dalam satu request agar beberapa file tidak kelebihan muatan di akun yang sama.

### API key (`api-key.routes.ts` + `api-key.middleware.ts`)
- Secret format: `9d_live_` + 32 random chars; `keyPrefix = secret.slice(0,16)`; `keyHash = hashToken(secret)` (SHA-256 hex).
- Middleware `requireApiKey(scope)`: parse `Authorization: Bearer <key>` → lookup by `keyHash` → cek status/revoked/expired → cek scope → set `req.user` (userId + `sessionId: "api-key:<id>"`) → update `lastUsedAt`.
- Scopes saat ini hanya `files:upload`.

### Storage breakdown (`storage.routes.ts`)
```sql
SELECT CASE
  WHEN mime_type LIKE 'image/%' THEN 'photo'
  WHEN mime_type LIKE 'video/%' THEN 'video'
  ELSE 'document'
END AS kind,
COALESCE(SUM(size_bytes), 0) AS bytes
FROM files
WHERE user_id = ? AND status = 'active'
GROUP BY kind
```

### Model database yang relevan (Prisma schema 9drive)
- `UploadRoutingPolicy`: userId (unique), mode (default `most_available`), priorityAccountIds (JSON), roundRobinCursor (Int).
- `ApiKey`: name, keyPrefix, keyHash (unique), scopes (JSON), status, lastUsedAt?, expiresAt?, revokedAt?.
- `StorageAccount`: totalBytes?, usedBytes, availableBytes?, trashBytes?, lastSyncedAt?.
- `AuditLog`: userId?, action, entityType, entityId?, metadata (JSON).
- `UploadSession`: targetConnectedAccountId?, folderId?, fileName, mimeType, sizeBytes, status, googleSessionUri?, errorMessage?, completedAt?.
- `FilePreviewToken`: fileId, userId, tokenHash (unique), expiresAt.
