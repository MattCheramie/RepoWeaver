# Architecture

RepoWeaver is a Go program that serves a server-rendered HTMX web UI and persists
everything to a single SQLite file. The frontend (templates + static assets) is
embedded into the binary via `embed.FS`, so a build is a single self-contained
executable.

```
                    ┌─────────────────────────────────────────────┐
   GitHub API ─────▶│ ingest ─▶ store(SQLite) ◀─ analyze ◀─ llm    │
   docs / PDFs ────▶│                │              │              │
                    │                ▼              ▼              │
                    │           server (net/http + html/template)  │
                    │            │ HTMX handlers, calendar, OAuth   │
                    │            ├─ seo (slugs, keywords, frontmatter)
                    │            └─ analytics (GA4 / demo)          │
                    └─────────────────────────────────────────────┘
                                     ▲
                          browser  ──┘  (or native webview window)
```

## Packages

| Package | Responsibility |
|---|---|
| `main` (+ `shell_web.go` / `shell_desktop.go`) | Loads config, opens the store, builds the server, binds a listener, and runs a **shell**. The shell is build-tag selected: default = headless web server; `desktop` = native webview window. |
| `internal/config` | Reads all configuration from environment variables, applying defaults. |
| `internal/store` | SQLite access (pure-Go `modernc.org/sqlite`). Owns the schema and all queries. |
| `internal/ingest` | Phase 1. Pulls GitHub history (`go-github`) and static files; extracts PDF text. |
| `internal/llm` | Pluggable LLM `Provider` interface and implementations (`mock`, `claude`, `openai`, `gemini`). |
| `internal/analyze` | Phase 2/3/4. Clustering + salvage and Markdown generation ("the Brain"). |
| `internal/seo` | Phase 4. Keyword density, slugs, AI-assisted meta/tags, YAML frontmatter. |
| `internal/analytics` | Phase 6. Pluggable analytics `Provider` (`ga4` via OAuth or service account, `demo`, `none`). |
| `internal/server` | HTTP routing, HTMX handlers, the editorial calendar, and the GA4 OAuth flow. |
| `web/` | `html/template` pages + HTMX fragments, and static CSS/JS (vendored HTMX, `charts.js`, `calendar.js`, `icon.svg`). |

## Request flow

1. `main` builds a `server.Server` with the store and the configured providers,
   binds `127.0.0.1:$PORT`, serves in a goroutine, and calls `runShell`.
2. Each page template defines a `content` block and is parsed together with
   `layout.html` into its own template set (so the `content` definitions don't
   collide). HTMX requests render named fragments (e.g. `clusters`,
   `seo-panel`, `calendar-root`) for partial swaps.
3. Handlers call into `ingest`, `analyze`, `seo`, `store`, and `analytics`.

## Data model (SQLite)

Defined in [`internal/store/schema.sql`](../internal/store/schema.sql); applied
idempotently on startup.

| Table | Key columns | Notes |
|---|---|---|
| `repos` | `owner, name, status, last_ingested_at` | `status`: `new` → `ingesting` → `ready` / `error`. Unique on `(owner, name)`. |
| `items` | `repo_id, kind, external_id, title, body, state, …` | One ingested unit. Unique on `(repo_id, kind, external_id)` so re-ingest upserts. |
| `clusters` | `repo_id, title, summary, narrative, target_format` | LLM-identified story; replaced wholesale on each analyze run. |
| `cluster_items` | `cluster_id, item_id` | Membership join. |
| `content` | `cluster_id, repo_id, title, format, body, seo_meta, status, scheduled_for` | Generated Markdown + SEO JSON + lifecycle. |
| `settings` | `key, value` | Generic KV; stores the GA4 OAuth token. |

**Item kinds:** `pr`, `issue`, `comment`, `commit`, `doc`, `pdf`.
**Target formats:** `blog`, `tutorial`, `video_script`, `deep_dive`.
**Content status:** `draft`, `scheduled`, `published` (derived from
`scheduled_for` relative to now).

## Ingestion

`internal/ingest` performs "total capture" but is bounded for responsiveness:

- **History** (`github.go`): issues + PRs (with state, incl. `merged`), comments,
  and commit messages. Up to `maxPages = 5` pages of `perPage = 50` each per
  resource. Works unauthenticated (low rate limit) or with `GITHUB_TOKEN`.
- **Files** (`files.go`): walks the default-branch tree for `CHANGELOG*`, files
  under `docs/` (`.md`/`.txt`/`.pdf`), and any `.pdf`. Up to `maxFiles = 40`
  files, each `≤ maxFileBytes = 512 KB`; PDF text via `ledongthuc/pdf`.
- Failures in one phase don't abort the other; items are upserted, then the repo
  is stamped `ready`.

## Provider interfaces (extension points)

**LLM** — [`internal/llm/llm.go`](../internal/llm/llm.go):

```go
type Provider interface {
    Complete(ctx context.Context, system, prompt string) (string, error)
    Name() string
}
```

`llm.New(cfg)` selects by `LLM_PROVIDER`, defaulting to `mock`. The `mock`
provider inspects the system prompt to return clustering JSON, SEO JSON, or
prose, which lets the whole pipeline run offline and deterministically.

**Analytics** — [`internal/analytics/analytics.go`](../internal/analytics/analytics.go):

```go
type Provider interface {
    Name() string
    Configured() bool
    Report(ctx context.Context, slugs []string) (map[string]Metrics, error)
}
```

The GA4 OAuth provider reads its token from a `TokenStore` (backed by the
`settings` table) on each request, so Connect/Disconnect take effect immediately.
Both GA4 auth paths share one `runReport` implementation.

See [`docs/DEVELOPMENT.md`](DEVELOPMENT.md#adding-a-provider) for how to add a new
LLM or analytics provider.
