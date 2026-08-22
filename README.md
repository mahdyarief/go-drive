# go-drive

**Self-hosted multi-tenant file management platform with an S3-compatible gateway.**

go-drive unifies multiple storage backends (S3, Google Drive, local filesystem) behind a single web UI and API. Each organization gets isolated storage, sharing controls, and programmatic access — deployable as one binary.

![go-drive](https://img.shields.io/badge/status-active-success) ![License](https://img.shields.io/badge/license-MIT-blue) ![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go) ![React](https://img.shields.io/badge/React-19-61DAFB?logo=react)

---

## What is go-drive?

go-drive is a **storage aggregation layer** — think of it as a self-hosted alternative to services like Filebase or Storj DCS, but with built-in file management, sharing, and multi-tenancy.

Instead of choosing one storage provider, go-drive lets you:

- **Connect multiple backends** — AWS S3, Cloudflare R2, Google Drive, local disk — and use them together
- **Route files intelligently** — set policies to send files to specific backends based on size, type, or cost tier
- **Access via S3 API** — any S3-compatible tool (`aws s3`, `rclone`, `s3cmd`, SDK) works out of the box
- **Share files securely** — time-limited links, password protection, download tracking
- **Stay in control** — self-hosted, single binary, your data on your infrastructure

### Who is it for?

- **Teams** that need shared file storage with per-project or per-department isolation
- **Developers** who want an S3-compatible API without managing raw bucket infrastructure
- **Agencies** managing files for multiple clients under one roof
- **Anyone** who wants Dropbox/Drive-like UX but on their own S3 or Google Drive account

---

## Key Features

### Storage
- **Multi-backend** — S3-compatible (AWS, R2, Wasabi, Backblaze B2, DigitalOcean Spaces, Hetzner Object Storage, IDrive e2, Storj), Google Drive, local filesystem
- **Routing policies** — send uploads to specific backends based on rules (size, type, cost tier)
- **Storage tiering** — automatically move files between hot and cold backends based on age/access
- **Quota management** — per-organization storage limits with enforcement

### File Management
- **Folder hierarchy** — nested folders with breadcrumbs and recent files
- **Tags** — label files with custom tags for cross-folder organization
- **Search** — full-text search across file names and metadata
- **Batch operations** — move or delete multiple files at once
- **Preview** — inline preview for images, PDFs, and documents

### Sharing & Links
- **Share links** — time-limited, password-protected download links
- **Upload links** — let external users upload files to your storage without an account
- **Tracked links** — monitor who accessed your files and when (analytics dashboard)

### Access & Integration
- **S3-compatible API** — full SigV4 authentication, works with `aws s3`, `rclone`, boto3, etc.
- **External upload API** — API-key authenticated endpoint for programmatic uploads
- **Webhooks** — real-time notifications on file events (upload, delete, share)
- **Google Drive sync** — ingest existing Drive files into your go-drive workspace

### Multi-Tenancy & Security
- **Schema-per-tenant isolation** — each organization's data is fully isolated at the database level
- **Role-based access** — owner, admin, member roles with granular permissions
- **Audit log** — track every action across the platform
- **Rate limiting** — per-tenant and per-endpoint rate limits
- **API keys** — scoped credentials for external integrations

---

## Quick Start

### Prerequisites

- **Go** 1.22+ ([install](https://go.dev/dl/))
- **Node.js** 20+ ([install](https://nodejs.org))
- **PostgreSQL** — Neon, Supabase, or any Postgres 14+ instance
- **Make** (pre-installed on macOS/Linux)

### 1. Clone and install

```bash
git clone https://github.com/mahdyarief/go-drive.git
cd go-drive
make install
```

### 2. Configure environment

```bash
cp server/.env.example server/.env
```

Edit `server/.env`:

```env
# Required
AUTHULA_SECRET=your-random-secret-here
DATABASE_URL=postgres://user:password@host:5432/godrive

# Optional
PORT=8081
AUTHULA_BASE_URL=http://localhost:8081
```

### 3. Run in development

```bash
make dev
```

This starts:
- **Go server** on `http://localhost:8081` (API + auth)
- **Vite dev server** on `http://localhost:5173` (React frontend with HMR)

Open `http://localhost:5173`, create an account, and you're ready to go.

### 4. Build for production

```bash
make build
./dist/server
```

One binary, one port (`:8081`), serves API + frontend. No Nginx, no separate static server.

---

## Database: PostgreSQL or SQLite?

go-drive supports two database backends — pick based on your scale:

| | SQLite | PostgreSQL |
|---|--------|------------|
| Setup | Zero — a file on disk | Requires a Postgres 14+ instance |
| Best for | Single server, small teams, self-hosting | Production, multi-instance, high traffic |
| Concurrent users | Up to ~10–20 concurrent users | 100+ concurrent users |
| Multi-tenancy | One `.db` file per tenant (`data/tenants/tenant_<slug>.db`) | Schema-per-tenant (`tenant_<slug>`) |
| Scaling | Single-node only | Horizontal scaling, replicas, managed services |

**Rule of thumb:**

- **100+ concurrent users: use PostgreSQL.** SQLite serializes writes (one writer at a time, even in WAL mode) and will start hitting lock contention under heavy concurrent upload load.
- **Up to ~10–20 concurrent users: SQLite is fine.** go-drive keeps transactions short (3-phase uploads, busy timeouts, connection pooling) so light concurrent workloads run smoothly on a single file — ideal for personal drives, small teams, and edge deployments (Fly.io volumes, VPS, Raspberry Pi).
- **In between?** Start with SQLite and migrate to PostgreSQL when you feel the write contention. Metadata is portable; blob storage backends (S3/GDrive/local) are configured per-tenant and unaffected.

### Using SQLite

```env
# server/.env
DB_DRIVER=sqlite
SQLITE_PATH=./data/app.db          # default
SQLITE_MAX_OPEN_CONNS=8            # default; tune for your workload
```

SQLite runs in WAL mode with `busy_timeout=60s` and `synchronous=NORMAL` out of the box — commits skip fsync for speed while checkpoints stay durable.

### Using PostgreSQL (default)

```env
# server/.env
DATABASE_URL=postgres://user:password@host:5432/godrive
```

Works with Neon, Supabase, Fly Postgres, RDS, or any Postgres 14+ instance. The connection pool defaults to 25 open / 5 idle connections.

---

## Using the S3 Gateway

go-drive exposes an S3-compatible API at `/api/s3`. Any tool that speaks S3 can connect:

### Configure AWS CLI

```bash
aws configure
# AWS Access Key ID: <your-api-key>
# AWS Secret Access Key: <your-api-secret>
# Default region: us-east-1
# Default output format: json

aws s3 ls --endpoint-url http://localhost:8081/api/s3
```

### Generate S3 credentials

1. Log in to the web UI
2. Go to **Settings → API Keys → S3 Keys**
3. Create a new key pair
4. Use the access key and secret in your S3 client

### Supported operations

- `ListBuckets` — `aws s3 ls`
- `ListObjectsV2` — `aws s3 ls s3://bucket/`
- `GetObject` — `aws s3 cp s3://bucket/key ./local-file`
- `PutObject` — `aws s3 cp ./local-file s3://bucket/key`
- `DeleteObject` — `aws s3 rm s3://bucket/key`
- `HeadObject` — `aws s3api head-object --bucket bucket --key key`
- Multipart uploads — `aws s3 cp` for large files

The bucket name maps to your organization's workspace. All objects are stored in your configured storage backend.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      React 19 Frontend                       │
│              (Vite + Tailwind + shadcn/ui)                   │
└──────────────────────┬──────────────────────────────────────┘
                       │ HTTP (same-origin in dev via Vite proxy)
                       ▼
┌─────────────────────────────────────────────────────────────┐
│                    Go/Gin API Server                         │
│  ┌──────────────┬──────────────┬──────────────────────────┐ │
│  │ Auth (Authula)│ Org/Tenant   │ S3 Gateway (SigV4)       │ │
│  │ Email+Pass   │ Middleware   │ File/Folder/Tag Handlers │ │
│  └──────────────┴──────────────┴──────────────────────────┘ │
└──────────────────────┬──────────────────────────────────────┘
                       │
        ┌──────────────┼──────────────┐
        ▼              ▼              ▼
┌──────────────┐ ┌──────────┐ ┌──────────────┐
│ Neon Postgres│ │ S3/MinIO │ │ Google Drive │
│ (metadata +  │ │ (objects)│ │ (objects)    │
│  multi-tenant│ └──────────┘ └──────────────┘
│  schemas)    │
└──────────────┘
```

### Multi-Tenancy Model

go-drive uses **schema-per-tenant** isolation:

- **Public schema** — users, sessions, organizations, memberships
- **Tenant schemas** (`tenant_<slug>`) — each org gets its own schema with files, folders, tags, links, etc.
- Requests are scoped via `SET LOCAL search_path` inside a transaction — no cross-tenant data leakage

### Storage Routing

Files can be routed to different backends based on:

- **Manual selection** — user picks the backend at upload time
- **Routing policy** — rules based on file size, MIME type, or custom tags
- **Tiering policy** — automatic migration from hot (S3) to cold (Backblaze B2) after N days

---

## Project Structure

```
.
├── server/                          # Go/Gin backend
│   ├── cmd/server/
│   │   ├── main.go                  # Entry point — wires DB, config, router
│   │   └── static/                  # Embedded frontend (populated by make build)
│   └── internal/
│       ├── config/                  # Postgres + Authula config
│       ├── handler/                 # HTTP handlers (one file per domain)
│       │   ├── s3gateway.go         # S3-compatible API (SigV4)
│       │   ├── file.go              # File CRUD + preview
│       │   ├── folder.go            # Folder hierarchy
│       │   ├── store.go             # Storage backend management
│       │   ├── share.go             # Share/upload/tracked links
│       │   ├── tiering.go           # Storage tiering automation
│       │   └── webhook.go           # Webhook delivery
│       ├── middleware/              # CORS, auth, tenant, rate limit
│       ├── model/                   # Bun ORM models
│       ├── migrate/                 # Schema migrations (public + tenant)
│       ├── store/                   # Data access layer
│       └── router/router.go         # Route registration
├── web/                             # React 19 frontend
│   └── src/
│       ├── components/              # UI components (shadcn/ui + custom)
│       ├── pages/                   # Route-level page components
│       ├── store/                   # Zustand stores (auth, org, files)
│       └── lib/                     # API client, i18n, utilities
├── Makefile                         # dev, build, install, clean
└── docs/                            # Deployment, database, troubleshooting
```

---

## API Overview

### Authentication

- **Session-based** — email/password login, Bearer token in `Authorization` header
- **S3 SigV4** — for `/api/s3/*` endpoints, using generated access keys
- **API keys** — scoped keys for external upload endpoint (`POST /api/v1/uploads`)
- **Public links** — token-authenticated share/upload/tracked links

### Endpoints

| Group | Path | Auth | Description |
|-------|------|------|-------------|
| Auth | `/auth/*` | None | Sign up, sign in, sign out (Authula) |
| Public | `/api/health`, `/api/shared/*` | None | Health check, public link access |
| S3 Gateway | `/api/s3/*` | SigV4 | S3-compatible object operations |
| External | `/api/v1/uploads` | API Key | Programmatic file upload |
| User | `/api/me`, `/api/orgs/*` | Session | Current user, organization management |
| Tenant | `/api/t/*` | Session + X-Org-Slug | Files, folders, tags, links, stores, webhooks |
| Admin | `/api/admin/*` | Session + Admin | System-wide user/org management |

Full API documentation is available at `/app/api-docs` when running the app.

---

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `AUTHULA_SECRET` | Yes | — | Session signing secret (use `openssl rand -hex 32`) |
| `DATABASE_URL` | Yes | — | Postgres connection string |
| `PORT` | No | `8081` | Server port |
| `AUTHULA_BASE_URL` | No | `http://localhost:8081` | Public URL for auth callbacks |

---

## Deployment

### Single Binary

```bash
make build
./dist/server
```

The binary embeds the frontend and serves everything on one port. Deploy it anywhere you can run a Go binary: VPS, Docker, Fly.io, Railway.

### Docker

```bash
docker build -t go-drive .
docker run -p 8081:8081 --env-file server/.env go-drive
```

### Fly.io

A `fly.toml` is included. Deploy with:

```bash
fly launch
fly deploy
```

See [docs/deployment.md](docs/deployment.md) for production checklist, reverse proxy setup, and TLS configuration.

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | [Go](https://go.dev) 1.22+ / [Gin](https://gin-gonic.com) |
| Auth | [Authula](https://github.com/Authula/authula) (email/password + sessions) |
| Database | [Neon Postgres](https://neon.tech) (schema-per-tenant isolation) |
| ORM | [Bun](https://bun.uptrace.dev) |
| Frontend | [React 19](https://react.dev) + [TypeScript](https://www.typescriptlang.org) (React Compiler) |
| Build | [Vite 8](https://vite.dev) |
| CSS | [Tailwind CSS v4](https://tailwindcss.com) |
| Components | [shadcn/ui](https://ui.shadcn.com) |
| State | [Zustand](https://zustand-demo.pmnd.rs) |
| Data Fetching | [TanStack Query](https://tanstack.com/query) |
| i18n | [react-i18next](https://react.i18next.com) |

---

## Development

```bash
make install        # Install Go + Node dependencies
make dev            # Run server (:8081) + web (:5173) in parallel
make dev-server     # Server only
make dev-web        # Web only
make build          # Production build → dist/server
make clean          # Remove build artifacts
```

### Adding a storage backend

1. Create a new file in `server/internal/store/` implementing the `StorageBackend` interface
2. Register it in `server/internal/handler/store.go`
3. Add UI for configuration in `web/src/pages/app/settings/StoresPage.tsx`

### Adding a new API endpoint

1. Create handler in `server/internal/handler/`
2. Register in `server/internal/router/router.go` (public, authed, or tenant group)
3. For tenant-scoped endpoints, use `tenant_tx` from Gin context

See [server/CLAUDE.md](server/CLAUDE.md) and [web/CLAUDE.md](web/CLAUDE.md) for detailed guidelines.

---

## Documentation

| Guide | Description |
|-------|-------------|
| [Deployment](docs/deployment.md) | Build, deploy (VPS, Docker, Fly.io), production checklist |
| [Database](docs/database.md) | Schema layout, tenant isolation, adding tables |
| [Troubleshooting](docs/troubleshooting.md) | Common issues, debugging tips, manual API testing |
| [Contributing](CONTRIBUTING.md) | Development workflow, code style, PR guidelines |

---

## Comparison

| Feature | go-drive | Nextcloud | MinIO | Filebase |
|---------|----------|-----------|-------|----------|
| Self-hosted | ✅ | ✅ | ✅ | ❌ |
| S3-compatible API | ✅ | ❌ | ✅ | ✅ |
| Multi-backend aggregation | ✅ | ❌ | ❌ | ❌ |
| Multi-tenant isolation | ✅ | ❌ | ❌ | ✅ |
| File sharing links | ✅ | ✅ | ❌ | ❌ |
| Upload links | ✅ | ❌ | ❌ | ❌ |
| Link analytics | ✅ | ❌ | ❌ | ❌ |
| Storage tiering | ✅ | ❌ | ❌ | ✅ |
| Webhooks | ✅ | ✅ | ✅ | ❌ |
| Single binary deploy | ✅ | ❌ | ❌ | N/A |
| Google Drive integration | ✅ | ❌ | ❌ | ❌ |

---

## License

[MIT](LICENSE) — use it however you want, commercial or personal.

---

## Credits

Built with Go, React, and too much coffee. Inspired by the need for a self-hosted storage solution that doesn't force you to choose between features and simplicity.
