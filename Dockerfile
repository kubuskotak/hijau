# syntax=docker/dockerfile:1
# Multi-stage build: Bun builds the SvelteKit SPA, Go builds a static server
# binary that serves both the REST API and that SPA, shipped on distroless.

# --- Stage 1: build the SPA ---
# Built on the BUILD platform (the SPA output is architecture-independent), so a
# multi-arch image build never emulates this stage under QEMU.
FROM --platform=$BUILDPLATFORM oven/bun:1.3.13 AS web
WORKDIR /app
# Workspace manifests + lockfile first for better layer caching.
COPY package.json bun.lock bunfig.toml ./
COPY apps/web apps/web
COPY apps/example apps/example
COPY apps/mcp apps/mcp
COPY packages packages
RUN bun install --frozen-lockfile
RUN cd apps/web && bun run --bun build

# --- Stage 2: build the Go binary (migrations are go:embed'd) ---
# Also built on the BUILD platform and cross-compiled to the target arch via
# GOARCH — CGO is off, so Go cross-compiles natively (no QEMU). TARGETOS/
# TARGETARCH are set by buildx for multi-arch; they default to the host for a
# plain `docker build` / `docker compose up --build`.
FROM --platform=$BUILDPLATFORM golang:1.26 AS api
WORKDIR /src
ARG TARGETOS
ARG TARGETARCH
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api ./
# TARGETARCH falls back to the build host's arch so a legacy (non-buildkit)
# builder — where the auto build-args aren't injected — still produces a valid
# binary instead of an empty GOARCH (which Go rejects as "linux/").
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-$(go env GOHOSTARCH)} go build -trimpath -o /out/hijau ./cmd/server

# --- Stage 3: runtime ---
FROM gcr.io/distroless/static-debian12 AS runtime
WORKDIR /app
COPY --from=api /out/hijau /app/hijau
COPY --from=web /app/apps/web/build /app/web
ENV PORT=8080 \
    HIJAU_WEB_DIR=/app/web \
    HIJAU_STORAGE_DIR=/app/data/screenshots
EXPOSE 8080
ENTRYPOINT ["/app/hijau"]
