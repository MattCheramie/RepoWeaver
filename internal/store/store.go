// Package store provides the SQLite persistence layer for RepoWeaver.
// It uses the pure-Go modernc.org/sqlite driver (no CGO).
package store

import (
	"database/sql"
	_ "embed"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// Store wraps the database handle.
type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the SQLite database at path and applies the
// schema. Foreign keys are enabled.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

// --- Repos ---

// AddRepo inserts a repo if absent and returns it. If it already exists the
// existing row is returned.
func (s *Store) AddRepo(owner, name string) (Repo, error) {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO repos (owner, name, added_at, status) VALUES (?, ?, ?, 'new')`,
		owner, name, time.Now().UTC(),
	)
	if err != nil {
		return Repo{}, err
	}
	return s.RepoByOwnerName(owner, name)
}

// RepoByOwnerName fetches a repo by owner/name.
func (s *Store) RepoByOwnerName(owner, name string) (Repo, error) {
	row := s.db.QueryRow(
		`SELECT id, owner, name, added_at, last_ingested_at, status FROM repos WHERE owner=? AND name=?`,
		owner, name)
	return scanRepo(row)
}

// RepoByID fetches a repo by id.
func (s *Store) RepoByID(id int64) (Repo, error) {
	row := s.db.QueryRow(
		`SELECT id, owner, name, added_at, last_ingested_at, status FROM repos WHERE id=?`, id)
	return scanRepo(row)
}

// ListRepos returns all repos, newest first.
func (s *Store) ListRepos() ([]Repo, error) {
	rows, err := s.db.Query(
		`SELECT id, owner, name, added_at, last_ingested_at, status FROM repos ORDER BY added_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Repo
	for rows.Next() {
		r, err := scanRepo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SetRepoStatus updates the status of a repo.
func (s *Store) SetRepoStatus(id int64, status string) error {
	_, err := s.db.Exec(`UPDATE repos SET status=? WHERE id=?`, status, id)
	return err
}

// MarkIngested sets last_ingested_at to now and status to "ready".
func (s *Store) MarkIngested(id int64) error {
	_, err := s.db.Exec(
		`UPDATE repos SET last_ingested_at=?, status='ready' WHERE id=?`,
		time.Now().UTC(), id)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRepo(sc scanner) (Repo, error) {
	var r Repo
	var last sql.NullTime
	if err := sc.Scan(&r.ID, &r.Owner, &r.Name, &r.AddedAt, &last, &r.Status); err != nil {
		return Repo{}, err
	}
	if last.Valid {
		r.LastIngestedAt = &last.Time
	}
	return r, nil
}

// --- Items ---

// UpsertItem inserts or updates an item, returning its id. The id is always
// looked up by the unique key afterwards, since LastInsertId is unreliable for
// ON CONFLICT DO UPDATE.
func (s *Store) UpsertItem(it Item) (int64, error) {
	_, err := s.db.Exec(`
		INSERT INTO items (repo_id, kind, external_id, title, body, state, author, url, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(repo_id, kind, external_id) DO UPDATE SET
			title=excluded.title, body=excluded.body, state=excluded.state,
			author=excluded.author, url=excluded.url, created_at=excluded.created_at`,
		it.RepoID, it.Kind, it.ExternalID, it.Title, it.Body, it.State, it.Author, it.URL, it.CreatedAt)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.db.QueryRow(
		`SELECT id FROM items WHERE repo_id=? AND kind=? AND external_id=?`,
		it.RepoID, it.Kind, it.ExternalID).Scan(&id)
	return id, err
}

// CountItemsByKind returns a map of kind -> count for a repo.
func (s *Store) CountItemsByKind(repoID int64) (map[string]int, error) {
	rows, err := s.db.Query(
		`SELECT kind, COUNT(*) FROM items WHERE repo_id=? GROUP BY kind`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return nil, err
		}
		out[k] = n
	}
	return out, rows.Err()
}

// ListItems returns items for a repo, optionally limited.
func (s *Store) ListItems(repoID int64, limit int) ([]Item, error) {
	q := `SELECT id, repo_id, kind, external_id, title, body, state, author, url, created_at
	      FROM items WHERE repo_id=? ORDER BY created_at DESC`
	args := []any{repoID}
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func scanItem(sc scanner) (Item, error) {
	var it Item
	var title, body, state, author, url sql.NullString
	var created sql.NullTime
	if err := sc.Scan(&it.ID, &it.RepoID, &it.Kind, &it.ExternalID,
		&title, &body, &state, &author, &url, &created); err != nil {
		return Item{}, err
	}
	it.Title, it.Body, it.State = title.String, body.String, state.String
	it.Author, it.URL = author.String, url.String
	if created.Valid {
		it.CreatedAt = created.Time
	}
	return it, nil
}

// --- Clusters ---

// ReplaceClusters deletes existing clusters for a repo and inserts new ones,
// each with its member item ids. Used when (re)running analysis.
func (s *Store) ReplaceClusters(repoID int64, clusters []Cluster, members [][]int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM clusters WHERE repo_id=?`, repoID); err != nil {
		return err
	}
	now := time.Now().UTC()
	for i, c := range clusters {
		res, err := tx.Exec(`
			INSERT INTO clusters (repo_id, title, summary, narrative, target_format, created_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			repoID, c.Title, c.Summary, c.Narrative, c.TargetFormat, now)
		if err != nil {
			return err
		}
		cid, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if i < len(members) {
			for _, itemID := range members[i] {
				if _, err := tx.Exec(
					`INSERT OR IGNORE INTO cluster_items (cluster_id, item_id) VALUES (?, ?)`,
					cid, itemID); err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit()
}

// ListClusters returns clusters for a repo with item counts.
func (s *Store) ListClusters(repoID int64) ([]Cluster, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.repo_id, c.title, c.summary, c.narrative, c.target_format, c.created_at,
		       (SELECT COUNT(*) FROM cluster_items ci WHERE ci.cluster_id=c.id) AS item_count
		FROM clusters c WHERE c.repo_id=? ORDER BY c.id`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Cluster
	for rows.Next() {
		var c Cluster
		var summary, narrative sql.NullString
		if err := rows.Scan(&c.ID, &c.RepoID, &c.Title, &summary, &narrative,
			&c.TargetFormat, &c.CreatedAt, &c.ItemCount); err != nil {
			return nil, err
		}
		c.Summary, c.Narrative = summary.String, narrative.String
		out = append(out, c)
	}
	return out, rows.Err()
}

// ClusterByID returns a single cluster.
func (s *Store) ClusterByID(id int64) (Cluster, error) {
	row := s.db.QueryRow(`
		SELECT c.id, c.repo_id, c.title, c.summary, c.narrative, c.target_format, c.created_at,
		       (SELECT COUNT(*) FROM cluster_items ci WHERE ci.cluster_id=c.id)
		FROM clusters c WHERE c.id=?`, id)
	var c Cluster
	var summary, narrative sql.NullString
	if err := row.Scan(&c.ID, &c.RepoID, &c.Title, &summary, &narrative,
		&c.TargetFormat, &c.CreatedAt, &c.ItemCount); err != nil {
		return Cluster{}, err
	}
	c.Summary, c.Narrative = summary.String, narrative.String
	return c, nil
}

// ClusterItems returns the items belonging to a cluster.
func (s *Store) ClusterItems(clusterID int64) ([]Item, error) {
	rows, err := s.db.Query(`
		SELECT i.id, i.repo_id, i.kind, i.external_id, i.title, i.body, i.state, i.author, i.url, i.created_at
		FROM items i JOIN cluster_items ci ON ci.item_id=i.id
		WHERE ci.cluster_id=? ORDER BY i.created_at`, clusterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// --- Content ---

// CreateContent inserts a generated content row and returns its id.
func (s *Store) CreateContent(c Content) (int64, error) {
	now := time.Now().UTC()
	res, err := s.db.Exec(`
		INSERT INTO content (cluster_id, repo_id, title, format, body, seo_meta, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 'draft', ?, ?)`,
		nullableInt(c.ClusterID), c.RepoID, c.Title, c.Format, c.Body, jsonOrEmpty(c.SEOMeta), now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateContentBody updates the markdown body of a content row.
func (s *Store) UpdateContentBody(id int64, body string) error {
	_, err := s.db.Exec(`UPDATE content SET body=?, updated_at=? WHERE id=?`,
		body, time.Now().UTC(), id)
	return err
}

// UpdateContentSEO replaces the seo_meta JSON of a content row.
func (s *Store) UpdateContentSEO(id int64, seoMeta string) error {
	_, err := s.db.Exec(`UPDATE content SET seo_meta=?, updated_at=? WHERE id=?`,
		jsonOrEmpty(seoMeta), time.Now().UTC(), id)
	return err
}

// SetSchedule sets (or clears, when t is nil) the publish date of a content
// row and derives its status: "draft" when unscheduled, "scheduled" for a
// future date, or "published" once the date has passed.
func (s *Store) SetSchedule(id int64, t *time.Time, now time.Time) error {
	status := "draft"
	if t != nil {
		if t.After(now) {
			status = "scheduled"
		} else {
			status = "published"
		}
	}
	_, err := s.db.Exec(`UPDATE content SET scheduled_for=?, status=?, updated_at=? WHERE id=?`,
		nullableTime(t), status, now, id)
	return err
}

// ListContent returns all content rows, newest first.
func (s *Store) ListContent() ([]Content, error) {
	rows, err := s.db.Query(`
		SELECT id, cluster_id, repo_id, title, format, body, seo_meta, status, scheduled_for, created_at, updated_at
		FROM content ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Content
	for rows.Next() {
		c, err := scanContent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ContentByID returns a single content row.
func (s *Store) ContentByID(id int64) (Content, error) {
	row := s.db.QueryRow(`
		SELECT id, cluster_id, repo_id, title, format, body, seo_meta, status, scheduled_for, created_at, updated_at
		FROM content WHERE id=?`, id)
	return scanContent(row)
}

func scanContent(sc scanner) (Content, error) {
	var c Content
	var clusterID sql.NullInt64
	var scheduled sql.NullTime
	if err := sc.Scan(&c.ID, &clusterID, &c.RepoID, &c.Title, &c.Format, &c.Body,
		&c.SEOMeta, &c.Status, &scheduled, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return Content{}, err
	}
	if clusterID.Valid {
		c.ClusterID = clusterID.Int64
	}
	if scheduled.Valid {
		c.ScheduledFor = &scheduled.Time
	}
	return c, nil
}

func nullableInt(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

func jsonOrEmpty(s string) string {
	if s == "" {
		return "{}"
	}
	return s
}
