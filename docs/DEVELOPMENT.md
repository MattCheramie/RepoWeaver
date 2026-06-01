# Development guide

## Prerequisites
- **Go 1.25+**. The default web build is **pure Go (no CGO)** thanks to the
  `modernc.org/sqlite` driver.
- The native desktop build additionally needs CGO + a system webview (see
  [Desktop build](#desktop-build)).

## Common tasks (Makefile)

| Command | What it does |
|---|---|
| `make run` | Run the web server (`go run .`). Defaults: `PORT=8080`, `LLM_PROVIDER=mock`. |
| `make build` | Build `./bin/repoweaver` (pure Go). |
| `make desktop` | Build the native window app (`-tags desktop`, needs CGO + webview). |
| `make dist` | Cross-compile release archives into `dist/`. |
| `make test` | `go test ./...` — runs fully offline via the mock/demo providers. |
| `make vet` | `go vet ./...`. |
| `make tidy` | `go mod tidy`. |
| `make clean` | Remove `bin/`, `dist/`, and `repoweaver.db`. |

## Running offline / for tests

The defaults make the whole pipeline deterministic and keyless:

- `LLM_PROVIDER=mock` — the mock provider inspects the system prompt and returns
  clustering JSON, SEO JSON, or prose as appropriate.
- `ANALYTICS_PROVIDER=demo` — fabricates stable per-slug metrics from a hash.

This is what the test suite uses, so `go test ./...` needs no network or keys.
A note on the build sandbox: outbound access may be restricted, so live GitHub
ingestion and live GA4 calls aren't exercised in CI — only their wiring is.

## Project layout

See [`docs/ARCHITECTURE.md`](ARCHITECTURE.md) for the package responsibilities and
data model. Source map:

```
main.go, shell_web.go, shell_desktop.go   entrypoint + web/desktop shells
internal/{config,store,ingest,llm,analyze,seo,analytics,server}
web/{templates,static}                     embedded UI
packaging/repoweaver.desktop               Linux launcher (release archives)
scripts/dist.sh                            release packaging
```

## HTTP routes

| Method & path | Handler | Purpose |
|---|---|---|
| `GET /` | repos | Dashboard: add a repo, list tracked repos. |
| `POST /repos` | add+ingest | Add `owner/name` and run ingestion (HTMX). |
| `GET /repos/{id}/hub` | hub | Knowledge Hub: item counts + clusters. |
| `POST /repos/{id}/analyze` | analyze | Run clustering; returns the `clusters` fragment. |
| `POST /clusters/{id}/generate` | generate | Generate Markdown + SEO for a cluster. |
| `GET /library` | library | All generated content. |
| `GET /content/{id}` | view | Preview/edit a content item + SEO panel. |
| `POST /content/{id}` | save | Save edited Markdown body. |
| `POST /content/{id}/seo` | recompute SEO | Returns the `seo-panel` fragment. |
| `POST /content/{id}/schedule` | schedule | Set/clear publish date; returns `calendar-root`. |
| `GET /content/{id}/download` | download | `.md` download; `?fm=1` prepends YAML frontmatter. |
| `GET /calendar` | calendar | Month grid; HTMX requests get the `calendar-root` fragment. |
| `GET /analytics` | dashboard | Performance dashboard or setup prompt. |
| `GET /analytics/connect` | OAuth start | Redirect to Google consent (CSRF state cookie). |
| `GET /analytics/oauth/callback` | OAuth callback | State-validated token exchange + save. |
| `POST /analytics/disconnect` | disconnect | Clear the stored GA4 token. |

## Desktop build

`shell_desktop.go` (`//go:build desktop`) wraps the embedded server in a native
window using `webview/webview_go`. The default build excludes it, so `go build
./...`, `go vet`, and tests stay pure-Go.

```bash
make desktop   # CGO_ENABLED=1 go build -tags desktop -o bin/repoweaver-desktop .
```

Webview prerequisites: Linux `libwebkit2gtk-4.1-dev` (or `4.0`) + GTK 3; macOS
WebKit (built in); Windows Edge **WebView2** runtime.

## Release packaging

`scripts/dist.sh` (via `make dist`) cross-compiles the pure-Go web build for
linux/{amd64,arm64}, darwin/{amd64,arm64}, and windows/amd64, then bundles each
target into a `tar.gz` (or `zip` on Windows) containing the binary, `README`,
`LICENSE`, `repoweaver.svg`, and — on Linux — `repoweaver.desktop`. It also writes
`SHA256SUMS`. The version (from `git describe`, or `make dist VERSION=...`) is
embedded via `-ldflags "-X main.version=..."` and reported at startup.

## Adding a provider

**LLM provider** — implement `llm.Provider` (`Complete`, `Name`) in a new file
under `internal/llm`, then add a case to `llm.New` keyed on `LLM_PROVIDER`. Look
at `internal/llm/openai.go` for a minimal REST example, and extend
`internal/llm/mock.go` if you want it covered by the offline tests.

**Analytics provider** — implement `analytics.Provider` (`Name`, `Configured`,
`Report`) under `internal/analytics` and wire it into `analytics.New` (and, for
auth that needs runtime state, the server's provider selection in
`internal/server/server.go`). Reuse `runReport` if you're targeting GA4.

## Testing notes
- `internal/store` — schema/migrations and CRUD round-trips on a temp DB.
- `internal/llm`, `internal/analytics` — provider selection, mock/demo behavior.
- `internal/seo` — keyword density, slugs, frontmatter, provider + fallback.
- `internal/server` — full HTTP pipeline (ingest-seed → analyze → generate →
  library → download), SEO recompute, calendar scheduling, the analytics
  dashboard, and the GA4 OAuth connect/callback/disconnect flow (CSRF included)
  — all via `httptest` with the mock/demo providers.
