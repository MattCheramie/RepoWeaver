# RepoWeaver

A centralized knowledge hub that extracts, synthesizes, and repurposes the
hidden intelligence inside an open-source repository's history and
documentation. RepoWeaver ingests a repository's entire lifecycle and turns that
raw public data into a multi-format content matrix — then serves as a complete
editorial suite to plan publishing schedules, optimize for search engines, and
track content performance.

It runs as a **local web application** (a Go server driving an HTMX + vanilla-JS
frontend) and can optionally be wrapped in a **native desktop window**.

> **Status: all six phases implemented.** The full loop works end to end —
> **ingest → analyze → generate → library → schedule → track** — and runs with
> zero external setup using the built-in keyless providers.

---

## Contents
- [What it does](#-what-it-does)
- [Quick start](#-quick-start)
- [Configuration](#-configuration)
- [Building & packaging](#-building--packaging)
- [Architecture](#-architecture)
- [Tech stack](#-tech-stack)
- [Workflow](#-workflow)
- [Docs](#-docs)
- [Notes & caveats](#-notes--caveats)

---

## 🎯 What it does

| Phase | Name | What's implemented |
|---|---|---|
| **1** | **Ingestion** (the Crawler) | Pulls PRs, issues, comments, and commit messages via the GitHub API, plus static `CHANGELOG`, `docs/` files, and `.pdf` reference documents. Total capture — bots, dependency bumps, and unresolved issues included. Cached in SQLite. |
| **2** | **Analysis** (the Brain) | An LLM maps the organic problem-solving history against the docs, clusters related items into story narratives, and runs a "salvage" pass over unresolved issues for usable snippets. |
| **3** | **Knowledge Hub** (the Editor) | Per-repo workspace listing item counts and the LLM-generated clusters, each tagged with a target format (blog, tutorial, video script, deep dive). |
| **4** | **Generation & SEO** (the Publisher) | Generates polished Markdown per cluster. Every post gets a programmatic header/hero banner plus charts and diagrams the subject matter calls for — all rendered as self-contained inline SVG (no image hosting). SEO toolkit: local keyword-density analysis, URL-slug suggestions, AI-assisted meta description + semantic tags, and YAML-frontmatter export. |
| **5** | **Library & Editorial Calendar** (the Planner) | Browse, preview, edit, and download generated `.md` (with or without frontmatter). A month-view calendar with **drag-and-drop** scheduling tracks `draft → scheduled → published`. |
| **6** | **Analytics & Tracking** (the Monitor) | A performance dashboard mapping pageviews, average time on page, and bounce rate onto scheduled/published posts, with a pageviews bar chart. Pulls from **Google Analytics 4** via browser **OAuth** or a **service account**. |

**Pluggable providers, runnable offline:**
- **LLM** — Anthropic **Claude** (default), **OpenAI**, Google **Gemini**, plus a
  keyless **`mock`** provider used by default and in tests.
- **Analytics** — **GA4** (OAuth or service account), a deterministic **`demo`**
  provider, or **none** (shows a setup prompt).

---

## 🏃 Quick start

Requires **Go 1.25+**. The default web build is **pure Go (no CGO)** — it uses
the `modernc.org/sqlite` driver.

```bash
# Zero setup: runs with the keyless mock LLM provider and the in-repo SQLite DB.
make run                       # serves http://localhost:8080
```

With real providers (a GitHub token is recommended to avoid API rate limits):

```bash
cp .env.example .env           # edit it, or export the vars directly
GITHUB_TOKEN=ghp_xxx LLM_PROVIDER=claude LLM_API_KEY=sk-ant-xxx make run
```

Then open <http://localhost:8080>, add a repository as `owner/name` (e.g.
`octocat/Hello-World`), and follow the workflow below.

```bash
make test    # full test suite — runs fully offline via the mock/demo providers
make vet     # go vet
make build   # build ./bin/repoweaver
```

---

## ⚙️ Configuration

All configuration is via environment variables (see [`.env.example`](.env.example)
and [`docs/CONFIGURATION.md`](docs/CONFIGURATION.md) for the full reference).

| Variable | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | HTTP port (bound on `127.0.0.1`). |
| `DB_PATH` | `./repoweaver.db` | SQLite database file. |
| `OPEN_BROWSER` | `false` | Open the default browser on startup. |
| `GITHUB_TOKEN` | — | GitHub PAT. Optional but strongly recommended (rate limits). |
| `LLM_PROVIDER` | `mock` | `mock` \| `claude` \| `openai` \| `gemini`. |
| `LLM_API_KEY` | — | API key for the selected LLM provider. |
| `LLM_MODEL` | per-provider | Override the model (defaults: `claude-sonnet-4-6`, `gpt-4o`, `gemini-1.5-pro`). |
| `ANALYTICS_PROVIDER` | `none` | `none` \| `ga4` \| `demo`. |
| `GA4_PROPERTY_ID` | — | Numeric GA4 property ID. |
| `GA4_OAUTH_CLIENT_ID` / `GA4_OAUTH_CLIENT_SECRET` | — | GA4 **browser OAuth** (Connect button on the dashboard). |
| `GA4_CREDENTIALS_FILE` / `GA4_CREDENTIALS_JSON` | — | GA4 **service-account** auth (no browser step). |

> **GA4 setup** (OAuth vs. service account) is documented step by step in
> [`docs/CONFIGURATION.md`](docs/CONFIGURATION.md#google-analytics-4). For a
> keyless preview of the dashboard, set `ANALYTICS_PROVIDER=demo`.

---

## 📦 Building & packaging

### Web build (default, pure Go)
```bash
make build     # ./bin/repoweaver
```

### Native desktop app (optional)
The same backend can run inside a native OS window via a system **webview**, with
no backend changes — selected by the `desktop` build tag. Requires **CGO** and a
platform webview:

- **Linux:** `libwebkit2gtk-4.1-dev` (or `4.0`) + GTK 3
- **macOS:** WebKit (built in)
- **Windows:** the Edge **WebView2** runtime

```bash
make desktop   # CGO_ENABLED=1 go build -tags desktop -o bin/repoweaver-desktop .
```

### Release archives
`make dist` cross-compiles the pure-Go web build for linux/{amd64,arm64},
darwin/{amd64,arm64}, and windows/amd64, writing archives to `dist/`:

```bash
make dist                 # version from `git describe`
make dist VERSION=v1.2.3  # explicit version, embedded via -ldflags
```

Each archive bundles the binary, `README`, `LICENSE`, the app icon
(`repoweaver.svg`), and — on Linux — a `repoweaver.desktop` launcher, alongside a
`SHA256SUMS` file. The running binary reports its version in the startup log.

See [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) for the full developer guide.

---

## 🏗️ Architecture

A Go backend serves server-rendered HTML with HTMX for interactivity; templates
and static assets are embedded into the binary. Persistence is a single SQLite
file. Full detail in [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

```
main.go                      entrypoint; embeds web/, binds the listener, picks the shell
shell_web.go / shell_desktop.go   default web shell vs. native webview (build-tagged)
internal/
  config/      env-driven configuration
  store/       SQLite layer + schema (repos, items, clusters, content, settings)
  ingest/      GitHub history + static files (CHANGELOG, docs/, PDFs)
  llm/         pluggable LLM providers (mock/claude/openai/gemini)
  analyze/     clustering, salvage, and content generation ("the Brain")
  seo/         keyword density, slugs, meta/tags, frontmatter
  analytics/   GA4 (OAuth + service account), demo, none
  server/      HTTP routing, HTMX handlers, editorial calendar, OAuth flow
web/
  templates/   html/template pages + HTMX fragments
  static/      css, vendored htmx.min.js, charts.js, calendar.js, icon.svg
```

**HTTP routes** (see [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md#http-routes)):
`GET /` · `POST /repos` · `GET /repos/{id}/hub` · `POST /repos/{id}/analyze` ·
`POST /clusters/{id}/generate` · `GET /library` · `GET|POST /content/{id}` ·
`POST /content/{id}/seo` · `POST /content/{id}/schedule` ·
`GET /content/{id}/download` · `GET /calendar` · `GET /analytics` ·
`GET /analytics/connect` · `GET /analytics/oauth/callback` ·
`POST /analytics/disconnect`.

---

## 🛠️ Tech stack

| Component | Technology | Notes |
|---|---|---|
| Backend / server | **Go** (stdlib `net/http`, `html/template`) | Method+pattern routing; no web framework. |
| Frontend | **HTMX** + vanilla JS | HTMX vendored; calendar drag-drop and the analytics chart are dependency-free vanilla JS. |
| App shell | local web server, or **webview** (`webview/webview_go`) | Native window behind the `desktop` build tag. |
| Storage | **SQLite** via `modernc.org/sqlite` | Pure-Go driver — no CGO for the web build. |
| GitHub | `google/go-github/v66` + `golang.org/x/oauth2` | History ingestion. |
| PDF | `ledongthuc/pdf` | Text extraction from `docs/*.pdf`. |
| LLM | Anthropic / OpenAI / Gemini REST (plain `net/http`) | Pluggable; keyless `mock` for offline. |
| Analytics | GA4 Data API (`runReport`) over REST + `golang.org/x/oauth2/google` | OAuth or service-account auth. |

---

## 🚀 Workflow

1. **Ingest & synthesize** — add a repo as `owner/name`; RepoWeaver pulls its
   history and docs, then **Analyze** clusters them into stories.
2. **Generate & optimize** — generate Markdown for a cluster, then review/recompute
   the SEO meta description, slug, tags, and keyword density.
3. **Schedule** — drag the post onto a date in the Editorial Calendar.
4. **Publish / export** — download the `.md` (optionally with YAML frontmatter)
   for your blog or static site.
5. **Track** — connect Google Analytics and watch pageviews, time on page, and
   bounce rate on the dashboard.

---

## 📚 Docs

- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — packages, request flow, data
  model, and provider interfaces / extension points.
- [`docs/CONFIGURATION.md`](docs/CONFIGURATION.md) — every environment variable
  and step-by-step Google Analytics 4 setup (OAuth and service account).
- [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) — build/test workflow, HTTP route
  reference, the desktop build, release packaging, and how to add a new provider.

---

## 📝 Notes & caveats

- **Runs offline by default.** `LLM_PROVIDER=mock` and `ANALYTICS_PROVIDER=demo`
  produce deterministic output so the whole app and its tests work with no API
  keys or network.
- **Ingestion is bounded** for responsiveness: up to ~5 pages each of issues/PRs,
  comments, and commits, and up to 40 docs/PDF files (≤512 KB each). See
  [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md#ingestion).
- **Analytics chart.** The dashboard chart uses a small self-contained Canvas
  renderer (`web/static/js/charts.js`) rather than Chart.js. It exposes the same
  data (`window.__repoweaverChart`), so swapping in Chart.js later is
  straightforward.
- **Post visuals are programmatic.** Hero banners and in-post charts are rendered
  to self-contained inline SVG (`internal/visual`), so they need no image hosting
  and render anywhere. Mermaid diagrams render client-side in Preview via a
  vendored bundle that ships as a documented stub (CDNs are blocked here); exports
  keep native ` ```mermaid ` fences. See [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md#post-visuals).
- **Live GA4** requires real credentials and outbound network; the provider
  wiring is unit-tested, but the live token exchange/report is exercised only
  against a configured account.

## 📄 License
See [`LICENSE`](LICENSE).
