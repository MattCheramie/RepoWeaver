# Configuration

RepoWeaver is configured entirely through environment variables. Copy
[`.env.example`](../.env.example) to `.env` and edit it, or export the variables
directly. Every value is optional — with none set, the app runs using the keyless
`mock` LLM provider and a local SQLite file.

## Reference

### Server
| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP port. The server binds `127.0.0.1` only. Set `PORT=0` to pick a free port. |
| `DB_PATH` | `./repoweaver.db` | SQLite database file path (created if absent). |
| `OPEN_BROWSER` | `false` | If `true`, open the default browser on startup (web build). |

### GitHub ingestion
| Variable | Default | Description |
|---|---|---|
| `GITHUB_TOKEN` | — | GitHub personal access token. Optional, but unauthenticated requests are heavily rate-limited; a token (even read-only/public scope) is strongly recommended. |

### LLM (analysis & generation)
| Variable | Default | Description |
|---|---|---|
| `LLM_PROVIDER` | `mock` | `mock` (keyless, deterministic), `claude`, `openai`, or `gemini`. |
| `LLM_API_KEY` | — | API key for the selected provider (not needed for `mock`). |
| `LLM_MODEL` | per-provider | Model override. Defaults: Claude `claude-sonnet-4-6`, OpenAI `gpt-4o`, Gemini `gemini-1.5-pro`. |

Example:
```bash
LLM_PROVIDER=claude LLM_API_KEY=sk-ant-... make run
```

### Analytics
| Variable | Default | Description |
|---|---|---|
| `ANALYTICS_PROVIDER` | `none` | `none`, `ga4`, or `demo`. |
| `GA4_PROPERTY_ID` | — | Numeric GA4 property ID (e.g. `123456789`). |
| `GA4_OAUTH_CLIENT_ID` | — | OAuth client ID for the browser sign-in flow. |
| `GA4_OAUTH_CLIENT_SECRET` | — | OAuth client secret. |
| `GA4_CREDENTIALS_FILE` | — | Path to a service-account JSON key file. |
| `GA4_CREDENTIALS_JSON` | — | Service-account JSON, inline (alternative to the file). |

Set `ANALYTICS_PROVIDER=demo` for a keyless dashboard with deterministic sample
metrics — useful for trying the UI without a Google account.

## Google Analytics 4

RepoWeaver reads pageviews, average session duration, and bounce rate by page
path over the last 28 days, then maps each row onto your posts by SEO **slug**
(the slug must appear in the page path on your live site). Choose **one** of the
two authentication methods below.

> Either way, set `ANALYTICS_PROVIDER=ga4` and `GA4_PROPERTY_ID=<your id>`.

### Option A — Browser OAuth (recommended for a single user)

1. In the [Google Cloud Console](https://console.cloud.google.com/), enable the
   **Google Analytics Data API**.
2. Create an **OAuth 2.0 Client ID** of type **Web application**.
3. Add the redirect URI:
   `http://localhost:8080/analytics/oauth/callback`
   (adjust the port if you changed `PORT`).
4. Configure and run:
   ```bash
   ANALYTICS_PROVIDER=ga4 \
   GA4_PROPERTY_ID=123456789 \
   GA4_OAUTH_CLIENT_ID=xxxxx.apps.googleusercontent.com \
   GA4_OAUTH_CLIENT_SECRET=xxxxx \
   make run
   ```
5. Open `/analytics` and click **Connect Google Analytics**. You'll grant
   read-only Analytics access and be returned to the dashboard. The token is
   stored in the `settings` table and refreshed automatically; **Disconnect**
   clears it.

### Option B — Service account (no browser step)

1. Enable the **Google Analytics Data API**.
2. Create a **service account** and download its JSON key.
3. In Google Analytics → **Admin → Property Access Management**, add the service
   account's email as a **Viewer** on the property.
4. Configure and run:
   ```bash
   ANALYTICS_PROVIDER=ga4 \
   GA4_PROPERTY_ID=123456789 \
   GA4_CREDENTIALS_FILE=/path/to/service-account.json \
   make run
   ```

If both OAuth and service-account credentials are present, the **service account
takes precedence**.
