# gin-react-monorepo — Umrah & Hajj Travel Management

Full-stack Go/Gin + React 19 monorepo with email/password auth (Authula), multi-tenant schema-per-tenant isolation on Neon Postgres, and single-binary deploy. Aplikasi khusus untuk mengelola manajemen travel umrah dan haji serta semua kebutuhan terkaitnya (PNR, tiket, jadwal penerbangan, embarkasi, paket perjalanan, dll).

## Quick Start

```bash
make install                        # Install Go + Node dependencies
cp server/.env.example server/.env  # Set AUTHULA_SECRET + DATABASE_URL
make dev                            # Runs server (:8081) + web (:5173)
```

Open `http://localhost:5173` to use the app. Create an account at `/signup`, then create an organization.

## Project Layout

```
.
├── server/          # Go/Gin backend (API + auth + embedded frontend)
│   ├── cmd/server/  # Entry point (main.go + embedded static/)
│   └── internal/    # config, handler, middleware, model, migrate, router
├── web/             # React 19 + TypeScript frontend (Vite + Tailwind + shadcn/ui) - React Compiler
│   └── src/         # App.tsx, pages/, store/, components/, lib/
└── Makefile         # dev, build, install, clean
```

See `server/CLAUDE.md` for Go-specific guidelines (architecture, patterns, adding endpoints).

## How Server and Web Communicate

In **development**, two processes run independently:
- Go server on `:8081` — serves `/api/*` and `/auth/*`
- Vite dev server on `:5173` — serves the React app with hot reload
- Vite proxies `/api` and `/auth` requests to `:8081` (configured in `web/vite.config.ts`)
- This makes everything same-origin, so session cookies work without CORS issues

In **production** (`make build`):
- Vite builds the React app to static files
- Static files are copied into `server/cmd/server/static/`
- Go embeds them via `//go:embed static/*`
- The single binary serves API + frontend on one port
- `handler/static.go` handles SPA routing (unknown paths → `index.html`)

## Web rules
Always use shadcn-ui skill on React web solution.

**No god files**: React pages must be split into small, single-responsibility components and modules. A page that grows past ~500 lines must be broken up — extract self-contained features (cards, dialogs, forms) into their own component files under `pages/<page>/`, and shared types/helpers into a colocated module (e.g. `pages/<page>/<page>.ts`). A page file should stay thin — mostly composition of its components.

## Multi-Tenancy Architecture

The app uses **schema-per-tenant** isolation on Neon Postgres:

- **Public schema** holds shared data: Authula tables (users, sessions), `organizations`, `organization_members`
- **Tenant schemas** (`tenant_<slug>`) hold per-org business data, isolated via `SET LOCAL search_path`
- Users are global (one account, many orgs)
- Each request to `/api/t/*` requires an `X-Org-Slug` header

### Request Flow (Tenant-Scoped)
1. CORS middleware allows `Authorization` and `X-Org-Slug` headers
2. Auth middleware extracts Bearer token, queries Authula's `sessions` table directly, sets `user_id` in context
3. Tenant middleware reads `X-Org-Slug`, verifies membership, begins transaction with `SET LOCAL search_path TO tenant_<slug>, public`
4. Handler uses `tenant_tx` from context for all DB queries
5. Tenant middleware commits or rolls back transaction

### Organization Lifecycle
- `POST /api/orgs` → creates org record + `tenant_<slug>` schema in one transaction
- `DELETE /api/orgs/:slug` → drops schema + deletes org record (owner only)

## Auth Flow (Hybrid: Cookie + Bearer)

The server uses Authula with email-password + session plugins. SPA uses httpOnly cookie (auto-sent, XSS-safe), API clients use Bearer token.

1. Web calls `POST /auth/sign-up` or `POST /auth/sign-in`
2. Authula validates credentials, creates a session, stores **SHA-256 hash** in DB
3. Authula sets `authula.session_token` cookie with **raw token** (via `SetCookie` hook)
4. Sign-in response returns `{ user, session: { token: "<hash>" } }` — hash disimpan di memory sebagai fallback Bearer
5. **SPA requests**: cookie dikirim otomatis (`credentials: 'include'`), middleware hash cookie value sebelum lookup
6. **API client requests**: `Authorization: Bearer <hash>` — hash sudah dalam bentuk DB, langsung match
7. Auth middleware fallback: Bearer header → cookie → hash jika dari cookie → query `sessions` table
8. `AuthLoader` selalu fetch `GET /api/me` (cookie auto-sent) untuk restore user + orgs
9. `ProtectedRoute` redirect ke `/login` jika `user` null; redirect admin ke `/admin`
10. Sign-out custom `POST /api/sign-out` hapus session via Bearer/cookie, clear cookie

**Important**: Authula stores SHA-256 hash in DB (`sha256.Sum256` → hex). Cookie contains raw token. Middleware hashes cookie value before lookup. Sign-in `session.token` in response body is already the hash — used directly as Bearer token.

**Architecture rationale**: Hybrid approach avoids XSS vulnerability of localStorage tokens while keeping Bearer token support for programmatic API clients. Cookie is httpOnly, SameSite, not accessible to JavaScript. Token in memory is only kept for the rare case where cookie is unavailable (e.g., some non-browser clients).

Database: Neon Postgres (shared `bun.DB` connection between Authula and app).

## API Routes

### Public (no auth)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/health` | Health check |
| GET | `/api/message` | Test message |

### Authenticated (session required)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/me` | Current user + organizations |
| POST | `/api/orgs` | Create organization |
| GET | `/api/orgs` | List user's organizations |
| GET | `/api/orgs/:slug` | Org details + members |
| PATCH | `/api/orgs/:slug` | Update org name (owner/admin) |
| DELETE | `/api/orgs/:slug` | Delete org + schema (owner) |
| POST | `/api/orgs/:slug/members` | Add member (owner/admin) |
| DELETE | `/api/orgs/:slug/members/:userId` | Remove member (owner/admin) |
| PATCH | `/api/orgs/:slug/members/:userId` | Change role (owner) |

### Tenant-Scoped (session + X-Org-Slug header)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/t/status` | Tenant context info |

## Key Files

| File | Purpose |
|------|---------|
| `server/cmd/server/main.go` | Entry point — wires DB, config, migrations, router |
| `server/internal/config/config.go` | Postgres connection + Authula config |
| `server/internal/router/router.go` | Mounts middleware, auth, API, org, tenant routes |
| `server/internal/handler/*.go` | HTTP handlers (health, user, org, tenant, static) |
| `server/internal/middleware/*.go` | CORS, auth, tenant middleware |
| `server/internal/model/*.go` | Bun models (Organization, OrganizationMember) |
| `server/internal/migrate/*.go` | Public schema + tenant schema migrations |
| `web/src/App.tsx` | React Router + AuthLoader (fetches user + orgs) |
| `web/src/store/auth.ts` | Auth state: signIn, signUp, signOut, user |
| `web/src/store/org.ts` | Org state: organizations, currentOrg, createOrg |
| `web/src/lib/api.ts` | Typed fetch wrapper + tenantApi with X-Org-Slug |
| `web/vite.config.ts` | Vite + Tailwind + proxy to Go server |

## Make Commands

| Command | What it does |
|---------|-------------|
| `make dev` | Run server + web in parallel (development) |
| `make dev-server` | Go server only on `:8081` |
| `make dev-web` | Vite dev server only on `:5173` |
| `make install` | `go mod download` + `npm install` |
| `make build` | Build single binary with embedded frontend → `dist/server` |
| `make clean` | Remove `dist/` and build artifacts |

## Environment Variables

Set in `server/.env` (copied from `.env.example`):

| Variable | Required | Default |
|----------|----------|---------|
| `AUTHULA_SECRET` | Yes | — |
| `DATABASE_URL` | Yes | — |
| `AUTHULA_BASE_URL` | No | `http://localhost:8081` |
| `PORT` | No | `8081` |

## Conventions

- **Go**: See `server/CLAUDE.md` for architecture, code style, and patterns
- **Frontend**: React 19, TypeScript strict, Tailwind CSS v4, shadcn/ui components
- **New API endpoints**: Create handler in `server/internal/handler/`, register in `router.go`
- **New tenant-scoped endpoints**: Add to `/api/t/` group in `router.go` (gets auth + tenant middleware)
- **New pages**: Create in `web/src/pages/`, add route in `App.tsx`
- **New UI components**: `cd web && npx shadcn@latest add <component>`
- **Protected routes**: Wrap with `<ProtectedRoute>` in `App.tsx`
- **Org-required routes**: Wrap with `<OrgGuard>` inside `<ProtectedRoute>`
- **New tenant tables**: Add CREATE TABLE to `migrate/tenant.go`, use `tenant_tx` from Gin context in handlers
