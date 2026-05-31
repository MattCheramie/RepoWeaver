# RepoWeaver 
A centralized knowledge hub designed to extract, synthesize, and repurpose the hidden intelligence within an open-source repository's history and documentation.
Operating as a standalone desktop web application, RepoWeaver ingests a repository's entire lifecycle and transforms that raw public data into a multi-format content matrix. Beyond generation, it serves as a complete editorial suite to plan publishing schedules, optimize for search engines, and track content performance.
## 🎯 Primary Objective
To serve as the central brain for repository intelligence and content management, autonomously executing the following:
 * **Discover & Ingest:** Pull all historical trial-and-error progressions, discussions, and static documentation.
 * **Analyze & Synthesize:** Cross-reference code changes with written docs to map the problem-solving narrative.
 * **Publish & Optimize:** Generate polished, SEO-optimized outputs (tutorials, deep dives, video scripts).
 * **Manage & Plan:** Store generated content in a browsable library and map out releases on an editorial calendar.
 * **Track:** Monitor post-publication performance via optional Google Analytics integration.
## 🗺️ Project Map & Architecture
The application utilizes a Go-powered backend wrapped in a lightweight Wails or Webview container, driving a dynamic HTMX + JavaScript frontend.
### Phase 1: Data Ingestion (The Crawler)
The backend pulls dynamic history via the GitHub API and parses static files directly from the repository's filesystem, caching it in SQLite.
 * **Targets:** PRs, issues, comments, commit messages, CHANGELOG, docs/, and .pdf reference documents.
 * **Scope:** Total capture. Includes bot PRs, dependency bumps, and unresolved issues for partial insights.
### Phase 2: AI Analysis & Extraction (The Brain)
The Go backend passes the combined context to an LLM API to extract narratives.
 * **Synthesis Prompting:** The LLM maps organic problem-solving against formal documentation.
 * **Salvage Operation:** Scans unresolved issues for usable code snippets or theoretical solutions.
### Phase 3: The Knowledge Hub (The Editor)
The core workspace where raw intelligence is organized.
 * **Clustering:** Automatically grouping related PRs and doc updates.
 * **Format Targeting:** Dictating whether a cluster becomes a tutorial, video script, or technical deep dive.
### Phase 4: Generation & SEO Optimization (The Publisher)
Generating the requested outputs with built-in search engine optimization.
 * **Multi-Format Output:** Technical blog posts, video scripts, developer insights, and step-by-step tutorials.
 * **SEO Toolkit:** AI-assisted meta-description generation, keyword density analysis, URL slug suggestions, and auto-generated semantic tags for blog post frontmatter.
### Phase 5: Content Library & Editorial Calendar (The Planner)
A persistent local system to manage the generated content lifecycle.
 * **Content Vault:** A browsable UI to store all generated posts. Users can preview, edit, and download/re-download the raw .md or .txt files at any time.
 * **Editorial Calendar:** A visual calendar interface (e.g., month/week views) to schedule upcoming posts, track drafted content, and log historical publish dates. Drag-and-drop functionality to adjust the timeline.
### Phase 6: Analytics & Tracking (The Monitor)
Closing the loop by tracking how the generated content actually performs in the wild.
 * **Google Analytics Integration (Optional):** OAuth connection to pull site data via the GA4 API.
 * **Performance Dashboard:** A dedicated UI page displaying pageviews, average time on page, and bounce rates directly mapped to the specific blog posts planned in the calendar.
## 🛠️ Tech Stack
| Component | Technology | Purpose |
|---|---|---|
| **Backend / Server** | Go | Handles file I/O, API interactions, LLM processing, and local server routing. |
| **App Shell** | Wails / Webview | Wraps the Go backend and frontend into a lightweight, native OS window. |
| **Frontend UI** | HTMX + Vanilla JS | Provides a reactive interface. JS handles the Calendar (e.g., FullCalendar) and Analytics charts (e.g., Chart.js). |
| **Data Parsing** | google/go-github, pdf | Interactions with the GitHub REST API and extraction of text from PDFs. |
| **Analytics integration** | google.golang.org/api/analyticsdata/v1beta | Pulls performance metrics from Google Analytics 4. |
| **Storage** | SQLite | Stores repository cache, generated markdown text, SEO metadata, and calendar dates. |
## 🚀 Execution Workflow
 1. **Ingest & Synthesize:** Point the application at the target repository to pull data and let the LLM map the concepts.
 2. **Review & Optimize:** Group the insights into topics, select the output format (e.g., Blog Post), and review the AI-suggested SEO keywords and meta descriptions.
 3. **Schedule:** Open the Editorial Calendar and drag the newly generated post onto a target publication date.
 4. **Publish / Export:** Download the optimized Markdown file from the Content Library to publish to your blog or static site.
 5. **Track:** If GA is connected, check the Analytics Dashboard a week later to see how much traffic the new post is driving.

## 🧪 Current Status (Walking-Skeleton MVP)
This repository currently implements an end-to-end **walking skeleton** of the
loop above: **ingest → analyze → generate → library → export**. It runs as a
plain local web server (the Wails native shell can wrap it later without backend
changes).

**Implemented**
- **Phase 1 — Ingestion:** GitHub PRs, issues, comments, and commit messages via
  the GitHub API, plus static `CHANGELOG`, `docs/`, and `.pdf` files. Cached in
  SQLite (total capture, including bots and unresolved issues).
- **Phase 2 — Analysis:** An LLM clusters related items into story narratives and
  runs a salvage pass over unresolved issues.
- **Phase 3 — Knowledge Hub:** Lists clusters per repo and drives generation.
- **Phase 4 — Generation & SEO:** Generates Markdown content, plus an SEO toolkit
  with local keyword-density analysis, URL-slug suggestions, AI-assisted
  meta-description and semantic-tag generation, and YAML-frontmatter export.
- **Phase 5 — Library & Editorial Calendar:** Browse, preview, edit, and download
  generated `.md` files (with or without frontmatter); a month-view calendar with
  drag-and-drop scheduling that tracks draft → scheduled → published status.
- **Pluggable LLM:** Anthropic Claude (default), OpenAI, and Google Gemini, plus
  a keyless `mock` provider so the whole app runs offline for development/tests.

**Stubbed (navigable placeholder page) for the final phase**
- Phase 6 Google Analytics 4 integration and performance dashboard (requires
  GA4 OAuth credentials and outbound network access).

## 🏃 Running Locally
Requires Go 1.25+ (no CGO — uses the pure-Go `modernc.org/sqlite` driver).

```bash
# Run with zero setup using the keyless mock LLM provider:
make run            # serves http://localhost:8080

# Or with a real provider and a GitHub token (recommended to avoid rate limits):
cp .env.example .env   # then edit it, or export the vars directly
GITHUB_TOKEN=ghp_xxx LLM_PROVIDER=claude LLM_API_KEY=sk-ant-xxx make run
```

Configuration is via environment variables — see [`.env.example`](.env.example).

```bash
make test    # full test suite (runs offline with the mock provider)
make vet     # go vet
make build   # build ./bin/repoweaver
```

**Project layout:** `main.go` (entrypoint + embedded web assets) ·
`internal/config` · `internal/store` (SQLite) · `internal/ingest` (GitHub +
files) · `internal/llm` (pluggable providers) · `internal/analyze` (clustering +
generation) · `internal/seo` (SEO toolkit) · `internal/server` (HTTP + HTMX
handlers, editorial calendar) · `web/` (templates + static assets).
