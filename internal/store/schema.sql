-- RepoWeaver schema. Idempotent: safe to run on every startup.

CREATE TABLE IF NOT EXISTS repos (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    owner            TEXT NOT NULL,
    name             TEXT NOT NULL,
    added_at         DATETIME NOT NULL,
    last_ingested_at DATETIME,
    status           TEXT NOT NULL DEFAULT 'new',
    UNIQUE(owner, name)
);

CREATE TABLE IF NOT EXISTS items (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id     INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL,
    external_id TEXT NOT NULL,
    title       TEXT,
    body        TEXT,
    state       TEXT,
    author      TEXT,
    url         TEXT,
    created_at  DATETIME,
    UNIQUE(repo_id, kind, external_id)
);

CREATE INDEX IF NOT EXISTS idx_items_repo ON items(repo_id);

CREATE TABLE IF NOT EXISTS clusters (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id       INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    title         TEXT NOT NULL,
    summary       TEXT,
    narrative     TEXT,
    target_format TEXT NOT NULL DEFAULT 'blog',
    created_at    DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_clusters_repo ON clusters(repo_id);

CREATE TABLE IF NOT EXISTS cluster_items (
    cluster_id INTEGER NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    item_id    INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    PRIMARY KEY (cluster_id, item_id)
);

CREATE TABLE IF NOT EXISTS content (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    cluster_id    INTEGER REFERENCES clusters(id) ON DELETE SET NULL,
    repo_id       INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    title         TEXT NOT NULL,
    format        TEXT NOT NULL,
    body          TEXT NOT NULL,
    seo_meta      TEXT NOT NULL DEFAULT '{}',
    status        TEXT NOT NULL DEFAULT 'draft',
    scheduled_for DATETIME,
    created_at    DATETIME NOT NULL,
    updated_at    DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_content_repo ON content(repo_id);

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- topics: subjects a repo touches on but does not cover in detail, identified
-- during analysis and researched (via the connected AI platform's live web
-- search) into reusable knowledge-base entries.
CREATE TABLE IF NOT EXISTS topics (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id       INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    rationale     TEXT,
    status        TEXT NOT NULL DEFAULT 'identified',
    -- identified | researching | researched | error | unsupported
    research      TEXT,
    sources       TEXT NOT NULL DEFAULT '[]', -- JSON: [{"title","url"}]
    error         TEXT,
    created_at    DATETIME NOT NULL,
    researched_at DATETIME,
    UNIQUE(repo_id, name)
);

CREATE INDEX IF NOT EXISTS idx_topics_repo ON topics(repo_id);
