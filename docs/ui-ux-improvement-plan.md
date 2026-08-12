# go-drive UI/UX & Layout Improvement Plan

> **Source reference**: [`D:\Github\reference\9drive`](D:\Github\reference\9drive) — frontend React/Vite storage gateway dashboard. Dipelajari per 2026-08-12 untuk menyerap pola UI/UX terbaik, dengan **go-drive sebagai base** dan **multi-tenancy (schema-per-tenant + OrgSwitcher)** sebagai constraint utama.

**Tujuan dokumen**: peta improvement UI/UX & layout untuk go-drive. Update progress tracker setiap kali batch selesai dikerjakan (setelah commit + push).

---

## 1. Perbandingan Layout Saat Ini

### go-drive (base — multi-tenant)
- **Sidebar kiri** (w-56, bisa collapse jadi w-16): `OrgSwitcher` di header sidebar, nav (Home, Dashboard, Files, Links, grup Settings: Stores/Members/Appearance/Notifications), user block (avatar inisial + nama + email + logout) di footer. Mobile: hamburger + drawer.
- **Main content**: `max-w-6xl p-4 md:p-6`, tanpa global header — search hanya ada di dalam FilesPage toolbar.
- **Upload progress**: `UploadProgressCard` — card inline di dalam FilesPage.
- **Theme**: light/dark/system via `SettingsAppearancePage` + `store/preferences.ts` (applyTheme).
- **FileList**: list/grid toggle (VIEW_MODE_KEY), FileItemActions dropdown, context menu sudah ada (onContextMenu). Belum ada drag-drop upload, details drawer, folder color/icon UI.

### 9drive (reference — single-tenant)
- **Sidebar kiri** (w-64, fixed, gradient branding): user profile block (gravatar + name + email), nav menu (All Files, Quota Tracker, Shared With Me, Starred, Recycle Bin, Activity Log, Setting, API Keys) dengan active-state `bg-blue-600/10`, dan **storage usage block pinned di bottom sidebar** (Photo/Video/Document/Free breakdown dengan colored dots + progress bar + Log Out button).
- **Global header di top**: search input dengan **advanced filters popover** (kind/account/size range/date range), theme toggle icon, system info dropdown (bell), dan **header actions yang di-inject oleh child pages** (via `useDriveLayoutActions().setHeaderActions`).
- **Upload progress panel**: fixed bottom-right, collapsible, per-file rows dengan percent/status/retry.
- **AllFilesPage**: drag-drop upload (drag overlay), folder grid dengan warna + ikon (FolderVisual), file details drawer, context menu file/folder/empty-area, folder size scale (xs/sm/md/lg), share/invite modal.
- **Mobile**: hamburger + drawer sidebar (sama).

---

## 2. Gap Analysis: Pola 9drive yang Bisa Diserap

| # | Pola 9drive | Status di go-drive | Nilai | Adaptasi multi-tenant |
|---|---|---|---|---|
| 1 | **Global header dengan search + advanced filters** | Search hanya di toolbar FilesPage | **Tinggi** | Search tetap tenant-scoped (`/api/t/files/search` + X-Org-Slug); header pakai OrgSwitcher context |
| 2 | **Storage usage block di sidebar** (quota bar + breakdown per kategori) | Hanya `StorageUsageCard` di FilesPage | **Tinggi** | Pakai `/api/t/storage/usage` per tenant; refresh saat org ganti |
| 3 | **Upload progress panel bottom-right** (fixed, collapsible, multi-entry, retry) | `UploadProgressCard` inline di FilesPage | **Tinggi** | Global overlay di AppLayout — relevan utk fitur #1 multi-upload (roadmap 9drive-absorption) |
| 4 | **Header actions injection** (child page bisa set tombol di header global) | Tidak ada | **Sedang** | `useOutletContext` pattern (React Router) |
| 5 | **Drag-drop upload** dengan overlay | Tidak ada | **Sedang** | Tenant-scoped upload handler sama |
| 6 | **Folder color + icon** (sudah ada `Color` field di model!) | Field `color` ada di model tapi UI belum pakai | **Sedang** | Color di per-tenant `folders` table |
| 7 | **File details drawer** (panel kanan: meta, ukuran, tanggal, akun) | Tidak ada | **Sedang** | Fetch-by-ID per tenant (sudah ada GetFile) |
| 8 | **Context menu lengkap** (file/folder/empty-area) | FileList sudah punya onContextMenu dasar | **Rendah** | — |
| 9 | **Theme toggle cepat di header** | Theme hanya di settings page | **Sedang** | Preferences store sudah global (bukan per-tenant) — aman |
| 10 | **System info / status dropdown** (connection status, storage engine) | Tidak ada (ada TenantStatusPage terpisah) | **Rendah** | Tampilkan status tenant aktif |

---

## 3. Roadmap Implementasi (urutan prioritas)

### Batch A — Layout shell (dampak paling terlihat)
- [x] **A1. Global header di AppLayout**
  - Tambah header bar di `web/src/components/app/AppLayout.tsx` (di atas `<main>` content, mobile-first): search input (tenant-scoped, navigasi ke `/app/files?q=...` atau pakai pattern FilesPage existing), theme toggle icon (Sun/Moon dari `store/preferences.ts`), dan slot header-actions via `useOutletContext`.
  - Tambah komponen baru `web/src/components/app/AppHeader.tsx` (bukan god file — AppLayout harus tetap tipis).
  - i18n keys baru di `en.json`/`id.json` (`app.searchPlaceholder`, dll).
- [x] **A2. Storage usage block di sidebar**
  - Tambah `StorageUsageSidebar.tsx` di footer sidebar (sebelum user block): quota bar (used/total dari `GET /api/t/storage/usage`), daftar store + used, tombol refresh.
  - Query key `['t','storage','usage',orgSlug]`; refetch saat `currentOrg` berubah (gunakan `useOrgStore`).
  - Mode collapse: sembunyikan teks, tampilkan hanya progress bar tipis.
- [x] **A3. Upload progress panel bottom-right**
  - Extend `UploadProgressCard` jadi global `UploadPanel` fixed bottom-right (collapsible, daftar file per-entry, status per-file, tombol retry). Tempatkan di `AppLayout` supaya muncul di semua halaman.
  - Ini jadi fondasi fitur #1 multi-upload (lihat `docs/9drive-absorption.md`).

### Batch B — File browsing
- [x] **B1. Drag-drop upload di FilesPage**
  - Drag overlay di container FilesPage (dragenter/dragleave/dragover/drop), pakai upload handler existing. Overlay: border dashed + teks "Drop files to upload".
- [x] **B2. Folder color + icon UI**
  - Model `folders` sudah punya `Color` (string hex). Cek apakah `icon_url` perlu ditambah di model + migration `server/internal/migrate/tenant.go`.
  - UI: color swatch picker di New Folder dialog + di grid tile folder (`FolderIcon` diwarnai `style={{ color }}`), mirip FolderVisual 9drive.
- [x] **B3. File details drawer**
  - Komponen `FileDetailsDrawer.tsx` di `pages/app/files/`: panel kanan (shadcn `Sheet`/Drawer) menampilkan meta file (nama, ukuran, tanggal, store, tags), aksi (download, rename, move, delete, share). Buka dari FileItemActions atau klik info.

### Batch C — Penyempurnaan
- [ ] **C1. Advanced search filters** (kind/size/date) — extend endpoint search atau filter client-side.
- [ ] **C2. System status dropdown** di header (ringkasan tenant: jumlah store, status sync, quota) — ganti/duplikat dari TenantStatusPage.
- [ ] **C3. Empty state + loading polish** di FilesPage (ikon besar + CTA, skeleton rows).

---

## 4. Constraint Multi-Tenancy (wajib dipatuhi)

1. **Semua query data tenant-scoped**: pakai `tenantApi` + `X-Org-Slug`; query key harus include `orgSlug` (`['t','files',orgSlug,...]`).
2. **OrgSwitcher tetap di sidebar header** — jangan dihapus/dipindah; ini pintu ganti tenant. Saat org berganti, semua query tenant (sidebar usage, header search, files) otomatis refetch via query key.
3. **Preferences (theme) global, bukan per-tenant** — `store/preferences.ts` sudah benar; jangan pindahkan ke per-org.
4. **i18n wajib** — semua string baru di `en.json` + `id.json`.
5. **No god files** — AppLayout tetap tipis; setiap fitur baru = komponen terpisah (AppHeader, StorageUsageSidebar, UploadPanel, FileDetailsDrawer).
6. **React Compiler**: no `Date.now()`/`Math.random()` di render body; no `useMemo`/`useCallback` manual; derived state dihitung saat render.
7. **base-ui**: prop `render`, bukan `asChild`; Select `onValueChange` null-coalesce.

---

## 5. Progress Tracker

| Batch | Item | Status | Commit | Catatan |
|---|---|---|---|---|
| A1 | Global header (search + theme toggle + actions slot) | ✅ | `06ca66c` | `AppHeader.tsx`; search → `/app/files?q=` |
| A2 | Storage usage block di sidebar | ✅ | `06ca66c` | `StorageUsageSidebar.tsx`; mode collapse = bar tipis |
| A3 | Upload progress panel bottom-right | ✅ | `06ca66c` | `UploadPanel.tsx` + `store/upload.ts`; fondasi multi-upload |
| B1 | Drag-drop upload | ✅ | `a7cf54c` | `FileDropZone.tsx`; dragDepth counter utk nested drag |
| B2 | Folder color + icon UI | ✅ | `a7cf54c` | `FOLDER_COLORS` + swatch picker + tint `FolderIcon`; tanpa `icon_url` |
| B3 | File details drawer | ✅ | `a7cf54c` | `FileDetailsDrawer.tsx` via base-ui Dialog kanan |
| C1 | Advanced search filters | ⬜ | — | — |
| C2 | System status dropdown | ⬜ | — | — |
| C3 | Empty state + loading polish | ⬜ | — | — |

**Status legend**: ⬜ Belum dimulai · 🔄 Sedang dikerjakan · ✅ Selesai (committed + pushed)

---

## 6. Referensi Implementasi

### Dari 9drive
- `layouts/DriveLayout.tsx` — pattern global header + sidebar storage block + upload panel + `setHeaderActions` outlet context.
- `components/drive/FolderVisual.tsx` — folder color/icon options + `normalizeFolderColor`.
- `context/UploadContext.tsx` — global upload state (files array, per-file percent/status, retry).
- `pages/AllFilesPage.tsx` — drag-drop overlay, folder size scale, details drawer, context menus.

### Dari go-drive (sudah ada)
- `components/app/AppLayout.tsx` — sidebar + collapse + mobile drawer + user block (tempat A1/A2/A3).
- `store/preferences.ts` — `applyTheme` + `theme` (untuk toggle cepat di header).
- `store/org.ts` — `currentOrg` (untuk refetch saat ganti tenant).
- `pages/app/files/files.ts` — `VIEW_MODE_KEY`, `formatBytes`, `ItemActions`.
- `pages/app/files/UploadProgressCard.tsx` — fondasi UploadPanel (A3).
- `pages/app/files/FileList.tsx` — sudah punya grid/list + context menu (dasar B2/B3).
- Model `folders.color` — sudah ada field warna di tenant schema (B2 tinggal UI).
