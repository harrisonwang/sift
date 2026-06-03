package cache

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/harrisonwang/sift/internal/model"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (no CGO)
)

// defaultQueryLimit caps Query results when Filter.Limit is unset.
const defaultQueryLimit = 200

// SQLite is the modernc.org/sqlite-backed Cache. A single underlying
// connection serializes writes, which keeps a short-lived CLI simple and
// immune to "database is locked" under concurrent provider fetches.
type SQLite struct {
	db *sql.DB
}

// Open opens (creating if needed) the SQLite database at path and initializes
// the schema. The caller is responsible for ensuring the parent directory
// exists (see config.EnsureDirs).
func Open(path string) (*SQLite, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	// One connection => writes are serialized; reads are quick for a CLI.
	db.SetMaxOpenConns(1)

	c := &SQLite{db: db}
	if err := c.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return c, nil
}

func (c *SQLite) initSchema() error {
	const ddl = `
CREATE TABLE IF NOT EXISTS items (
    id           TEXT PRIMARY KEY,
    provider     TEXT NOT NULL,
    source       TEXT NOT NULL,
    external_id  TEXT NOT NULL,
    title        TEXT NOT NULL DEFAULT '',
    url          TEXT NOT NULL DEFAULT '',
    author       TEXT NOT NULL DEFAULT '',
    summary      TEXT NOT NULL DEFAULT '',
    published_at TEXT NOT NULL DEFAULT '',
    fetched_at   TEXT NOT NULL,
    fetched_day  TEXT NOT NULL,
    score        INTEGER NOT NULL DEFAULT 0,
    tags         TEXT NOT NULL DEFAULT '',
    extra        TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_items_day      ON items(fetched_day);
CREATE INDEX IF NOT EXISTS idx_items_provider ON items(provider);
CREATE INDEX IF NOT EXISTS idx_items_source   ON items(source);
`
	if _, err := c.db.Exec(ddl); err != nil {
		return fmt.Errorf("init schema: %w", err)
	}
	return nil
}

func (c *SQLite) InsertIfNew(item model.Item) (bool, error) {
	now := time.Now()
	if item.FetchedAt.IsZero() {
		item.FetchedAt = now
	}

	tags := ""
	if len(item.Tags) > 0 {
		if b, err := json.Marshal(item.Tags); err == nil {
			tags = string(b)
		}
	}
	extra := ""
	if len(item.Extra) > 0 {
		if b, err := json.Marshal(item.Extra); err == nil {
			extra = string(b)
		}
	}

	res, err := c.db.Exec(`
		INSERT OR IGNORE INTO items
		(id, provider, source, external_id, title, url, author, summary,
		 published_at, fetched_at, fetched_day, score, tags, extra)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		item.ID(), item.Provider, item.Source, item.ExternalID,
		item.Title, item.URL, item.Author, item.Summary,
		formatTime(item.PublishedAt), item.FetchedAt.UTC().Format(time.RFC3339),
		item.FetchedAt.Local().Format("2006-01-02"),
		item.Score, tags, extra,
	)
	if err != nil {
		return false, fmt.Errorf("insert item: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// whereClause builds the shared SQL filter fragment (with a leading " WHERE ",
// or "" when no conditions apply) and its bind args, for Query and Delete.
func whereClause(filter Filter) (string, []any) {
	var (
		conds []string
		args  []any
	)
	if filter.Date != "" {
		conds = append(conds, "fetched_day = ?")
		args = append(args, filter.Date)
	}
	if filter.Provider != "" {
		conds = append(conds, "provider = ?")
		args = append(args, filter.Provider)
	}
	if filter.Source != "" {
		conds = append(conds, "source = ?")
		args = append(args, filter.Source)
	}
	if filter.Keyword != "" {
		conds = append(conds, "(LOWER(title) LIKE ? OR LOWER(summary) LIKE ?)")
		kw := "%" + strings.ToLower(filter.Keyword) + "%"
		args = append(args, kw, kw)
	}
	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func (c *SQLite) Query(filter Filter) ([]model.Item, error) {
	where, args := whereClause(filter)

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultQueryLimit
	}

	q := `SELECT provider, source, external_id, title, url, author, summary,
	             published_at, fetched_at, score, tags, extra
	      FROM items` + where +
		" ORDER BY published_at DESC, fetched_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := c.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("query items: %w", err)
	}
	defer rows.Close()

	var out []model.Item
	for rows.Next() {
		var (
			it                 model.Item
			published, fetched string
			tags, extra        string
		)
		if err := rows.Scan(
			&it.Provider, &it.Source, &it.ExternalID, &it.Title, &it.URL,
			&it.Author, &it.Summary, &published, &fetched, &it.Score,
			&tags, &extra,
		); err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}
		it.PublishedAt = parseTime(published)
		it.FetchedAt = parseTime(fetched)
		if tags != "" {
			_ = json.Unmarshal([]byte(tags), &it.Tags)
		}
		if extra != "" {
			_ = json.Unmarshal([]byte(extra), &it.Extra)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (c *SQLite) Delete(filter Filter) (int64, error) {
	where, args := whereClause(filter)
	res, err := c.db.Exec("DELETE FROM items"+where, args...)
	if err != nil {
		return 0, fmt.Errorf("delete items: %w", err)
	}
	return res.RowsAffected()
}

func (c *SQLite) Sources() ([]string, error) {
	rows, err := c.db.Query("SELECT DISTINCT source FROM items ORDER BY source")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (c *SQLite) Close() error { return c.db.Close() }

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}
