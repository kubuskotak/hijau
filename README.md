# Hijau

Self-hosted, developer-first **software localization platform** — a fully open-source (MIT)
alternative to Crowdin / Phrase / Tolgee.

Two things set it apart:

- **In-context translation editing** — drop the JS SDK into your app and edit any visible
  string *inside the running product* (including production) with an ALT/Option-click flow,
  capturing one-click screenshots for translators.
- **AI-native interface** — a REST API plus an **MCP server** so coding assistants can manage
  translations, and a CLI for CI/CD.

Around those: teams/projects, languages, translation keys, a clean editor UI, machine-translation
suggestions, translation memory, glossary, comments, review workflow, activity log, and per-string
history.

## Stack

| Layer | Tech |
|---|---|
| Backend | Go · [espresso](https://github.com/suryakencana007/espresso) · sqlc · pgx · Postgres |
| Web UI | SvelteKit · shadcn-svelte (Svelte 5 / Tailwind v4 / Bits UI) · Bun |
| SDK | TypeScript (`@hijau/*`) — framework-agnostic core + Svelte/React bindings |
| MCP server | TypeScript · `@modelcontextprotocol/sdk` |
| CLI | Go (static binary) |
| Monorepo | moon (tasks) · mise (toolchain) |

## Develop

```sh
mise install              # provision Go, Bun, sqlc
cp .env.example .env       # then edit secrets
docker compose up -d postgres
moon run :dev              # Postgres + Go API + web dev server
```

## Layout

```
apps/api    Go backend (espresso + sqlc + pgx)
apps/web    SvelteKit + shadcn-svelte
apps/mcp    MCP server (TypeScript)
apps/cli    hijau CLI (Go)
packages/   shared TS: i18n (ICU + marker codec), sdk-core, sdk-incontext,
            sdk-svelte, sdk-react, api-client (generated from OpenAPI)
```

See the full plan and milestone sequencing in the project plan file.

## License

MIT
