# ============================================================
# go-drive — multi-stage build
# Stage 1: build React frontend (web/dist)
# Stage 2: build Go server binary with embedded frontend
# Stage 3: minimal runtime image
# ============================================================

# ---------- Stage 1: Frontend ----------
FROM node:22-alpine AS web

WORKDIR /app/web

# Install dependencies first (cache-friendly layer)
COPY web/package.json web/package-lock.json ./
RUN npm ci

# Build frontend
COPY web/ ./
RUN npm run build

# ---------- Stage 2: Go server ----------
FROM golang:1.26-alpine AS server

WORKDIR /app

# Cache Go modules first
COPY server/go.mod server/go.sum ./
RUN go mod download

# Copy server source, then overlay built frontend into embedded static/
COPY server/ ./
COPY --from=web /app/web/dist ./cmd/server/static/

# Static single binary — app uses only pure-Go deps (lib/pq, bun, oauth2)
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server-bin ./cmd/server

# ---------- Stage 3: Runtime ----------
FROM alpine:3.22

# ca-certificates: HTTPS to Neon Postgres + Google Drive API
# tzdata: correct local time for payment filenames / date displays
# curl: healthcheck
RUN apk add --no-cache ca-certificates tzdata curl

WORKDIR /app
COPY --from=server /app/server-bin /app/server

# godotenv.Load() reads /app/.env if present (mounted by docker-compose)
ENV PORT=8081

EXPOSE 8081

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -fsS http://localhost:8081/api/health >/dev/null || exit 1

USER nobody
CMD ["/app/server"]
