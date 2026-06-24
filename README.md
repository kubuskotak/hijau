# Hijau

**Open-source, self-hosted software localization — a developer-first alternative to Crowdin / Phrase / Tolgee, fully OSS (MIT, not open-core).**

Hijau is a complete translation-management system built around two things most OSS tools do poorly:

- **In-context / in-production editing** — drop a tiny JS SDK into your running app and edit any visible string in place (Alt/Option-click → overlay editor → save), with one-click screenshots captured for translators.
- **An AI-native interface** — a REST API, an **MCP server** so coding assistants (Claude, Cursor, …) can manage translations, and a **CLI** for CI/CD.

Around those it’s a full TMS: teams & per-language roles, projects, languages, keys, a keyboard-friendly editor, machine translation, translation memory, glossary, auto-translate, review workflow, comments, per-string history, a live activity feed, webhooks, and import/export to common formats — and it self-hosts with **one command**.

---

## Quickstart (Docker)

```sh
cp .env.example .env          # then set SESSION_SECRET + HIJAU_ENCRYPTION_KEY
docker compose up --build     # Postgres + the app (REST API + web UI) on :8080
```

Open **http://localhost:8080**, sign up, and create a project. Migrations run automatically on boot, and the Go binary serves the built SPA — one container, no extra web server.

> Generate the secrets with high entropy, e.g. `openssl rand -base64 32`. `HIJAU_ENCRYPTION_KEY` encrypts MT/webhook credentials at rest (AES-256-GCM), so keep it stable across restarts.

Once you’re in, everything is **self-service from the UI** — no curl needed:

- **Settings** (top nav) → create personal access tokens for the CLI and MCP server.
- **project → Settings** → configure the machine-translation provider + key.
- **project → Members** → invite teammates by email and scope translators/reviewers to specific languages.

---

## What’s inside

| Area | Capability |
|------|------------|
| **Core** | Orgs, projects, languages, namespaces, keys, translations. Atomic write path: state machine (untranslated → translated → reviewed, + needs-work / outdated), per-string history, activity, and base-edit OUTDATED cascade — all in one transaction. |
| **Editor** | Keys × languages grid, inline edit, approve/reject, ICU placeholder validation, search; side panel with history, comments, screenshots, and **MT/TM suggestions**. Live updates over SSE. |
| **In-context** | `@hijau/web` (framework-agnostic), `@hijau/incontext` (Alt-click scanner + Shadow-DOM overlay + screenshots), `@hijau/react`, `@hijau/svelte`. A read-only **editor token** is safe to ship in a browser; writing requires unlock (re-auth) and is attributed to the real user. |
| **AI-native** | REST API · **MCP server** (`@hijau/mcp`, stdio) · **Go CLI** (`hijau`). |
| **Intelligence** | Machine translation (Claude by default, pluggable) with an **ICU placeholder guard**; translation memory (exact + fuzzy via `pg_trgm`, populated on approval); glossary (injected into MT prompts); **auto-translate** (TM → glossary → MT → validate). |
| **Collaboration** | Per-language roles (owner/admin/developer/translator/reviewer), comments, history, **activity feed**, **HMAC-signed webhooks** with a delivery log. |
| **Formats** | Import/export: JSON (flat + nested/i18next), CSV, Android `strings.xml`, Apple `.strings`, XLIFF 1.2, PO/gettext. |
| **Ops** | One-command Docker self-host (the Go binary serves the built SPA), migrate-on-boot, AES-256-GCM-sealed provider credentials. |

---

## Stack

| Layer | Tech |
|---|---|
| Backend | Go · [espresso](https://github.com/suryakencana007/espresso) · sqlc · pgx · Postgres |
| Web UI | SvelteKit · shadcn-svelte (Svelte 5 / Tailwind v4 / Bits UI) · Bun |
| SDK | TypeScript (`@hijau/*`) — framework-agnostic core + Svelte/React bindings |
| MCP server | TypeScript · `@modelcontextprotocol/sdk` |
| CLI | Go (static binary) |
| Monorepo | moon (tasks) · mise (toolchain) |

```
apps/api    Go backend (espresso + sqlc + pgx); also serves the built SPA
apps/web    SvelteKit + shadcn-svelte UI
apps/mcp    MCP server (TypeScript)
apps/cli    hijau CLI (Go)
apps/example  sample storefront wired with the in-context SDK
packages/   shared TS: i18n (ICU + zero-width marker codec), sdk-core (@hijau/web),
            sdk-incontext, sdk-svelte, sdk-react
```

The Go `internal/service` layer is the keystone — REST handlers, the CLI (via REST), and the MCP server all reuse it, and auth resolves to one model so every entry point authorizes identically.

---

## Develop

Prereqs: [mise](https://mise.jdx.dev) (provisions Go, Bun, sqlc, moon) and Docker.

```sh
mise install                          # toolchain (go, bun, sqlc, moon)
cp .env.example .env                  # then edit secrets
docker compose up -d postgres         # Postgres
moon run :dev                         # Postgres + Go API (:8080) + web dev server
```

Or run pieces directly:

```sh
cd apps/api && go run ./cmd/server    # API on :8080 (migrate-on-boot)
cd apps/web && bun run dev            # Vite dev server, proxies /api → :8080
```

- **Backend**: edit `migrations/*.sql` + `queries/*.sql`, then `sqlc generate`; `go build ./... && go test ./...`.
- **Frontend / packages**: `bun run check` / `bun test` (Bun-only; tool binaries run via `bun --bun`).

---

## Using the pieces

**In-context SDK** — point it at your project and a read-only editor token:

```ts
import { createHijau } from '@hijau/web';
import { enableInContext } from '@hijau/incontext';

const hijau = createHijau({ language: 'en', records, apiUrl: '/api/v1', projectId: '…' });
enableInContext(hijau, { token: '<read-only editor token>' }); // Alt/Option-click to edit
```

**MCP server** — create a personal access token in **Settings**, then run:

```sh
HIJAU_API_URL=http://localhost:8080 HIJAU_TOKEN=hj_pat_… bun run apps/mcp/src/index.ts
```

**CLI**:

```sh
hijau login --api http://localhost:8080 --token hj_pat_…
hijau status --project <id>
hijau pull --project <id> --dir ./locales            # export per-language JSON
hijau push --project <id> --lang fr --dir ./locales  # upsert
hijau extract ./src --check --project <id>           # CI gate: keys used but missing
```

---

## Configuration

| Env | Purpose |
|-----|---------|
| `DATABASE_URL` | Postgres DSN (required) |
| `PORT` | HTTP port (default 8080) |
| `PUBLIC_URL` | external URL (cookie `Secure` flag, links) |
| `SESSION_SECRET` | session signing |
| `HIJAU_ENCRYPTION_KEY` | AES-256-GCM key for MT/webhook secrets — **high-entropy random**, kept stable |
| `HIJAU_STORAGE` / `HIJAU_STORAGE_DIR` | screenshot storage backend + dir |
| `CORS_ORIGINS` | comma-separated allowed origins (for the in-context SDK) |

---

## License

MIT — see [LICENSE](LICENSE).
