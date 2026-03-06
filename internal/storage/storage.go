package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Page struct {
	ID           int64
	URL          string
	Domain       string
	Title        string
	Status       string
	Depth        int
	HTMLPath     string
	MDPath       string
	MDMethod     string
	CrawledAt    time.Time
	FilterReason string
}

type SearchResult struct {
	ID        int64
	URL       string
	Domain    string
	Title     string
	CrawledAt time.Time
	Snippet   string
}

type DB struct {
	db *sql.DB
}

func New(dbPath string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate db: %w", err)
	}

	return &DB{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS pages (
			id INTEGER PRIMARY KEY,
			url TEXT UNIQUE,
			domain TEXT,
			title TEXT,
			status TEXT,
			depth INTEGER,
			html_path TEXT,
			md_path TEXT,
			md_method TEXT,
			crawled_at TIMESTAMP,
			filter_reason TEXT
		)
	`)
	if err != nil {
		return fmt.Errorf("create pages table: %w", err)
	}

	_, err = db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS pages_fts USING fts5(
			url UNINDEXED, domain UNINDEXED, title, body
		)
	`)
	if err != nil {
		return fmt.Errorf("create fts table: %w", err)
	}

	return nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) GetPage(url string) (*Page, error) {
	row := d.db.QueryRow(
		`SELECT id, url, domain, title, status, depth, html_path, md_path, md_method, crawled_at, filter_reason
		 FROM pages WHERE url = ?`, url)

	p := &Page{}
	var crawledAt sql.NullTime
	err := row.Scan(&p.ID, &p.URL, &p.Domain, &p.Title, &p.Status, &p.Depth,
		&p.HTMLPath, &p.MDPath, &p.MDMethod, &crawledAt, &p.FilterReason)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if crawledAt.Valid {
		p.CrawledAt = crawledAt.Time
	}
	return p, nil
}

func (d *DB) UpsertPage(page *Page) error {
	_, err := d.db.Exec(
		`INSERT INTO pages (url, domain, title, status, depth, html_path, md_path, md_method, crawled_at, filter_reason)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(url) DO UPDATE SET
			domain=excluded.domain, title=excluded.title, status=excluded.status,
			depth=excluded.depth, html_path=excluded.html_path, md_path=excluded.md_path,
			md_method=excluded.md_method, crawled_at=excluded.crawled_at, filter_reason=excluded.filter_reason`,
		page.URL, page.Domain, page.Title, page.Status, page.Depth,
		page.HTMLPath, page.MDPath, page.MDMethod, page.CrawledAt, page.FilterReason,
	)
	return err
}

func (d *DB) InsertFTS(id int64, url, domain, title, body string) error {
	// Delete any existing entry first (for resume/update scenarios)
	_, _ = d.db.Exec(`DELETE FROM pages_fts WHERE rowid = ?`, id)
	_, err := d.db.Exec(
		`INSERT INTO pages_fts(rowid, url, domain, title, body) VALUES(?, ?, ?, ?, ?)`,
		id, url, domain, title, body,
	)
	return err
}

func (d *DB) Search(query string, domain string, limit int) ([]SearchResult, error) {
	var (
		rows *sql.Rows
		err  error
	)

	// Quote each term so FTS5 special characters are treated as literals
	ftsQuery := quoteFTSQuery(query)

	baseQuery := `SELECT f.rowid, f.url, f.domain, f.title, p.crawled_at,
		snippet(pages_fts, 3, '', '', '...', 30)
		FROM pages_fts f
		LEFT JOIN pages p ON p.url = f.url
		WHERE pages_fts MATCH ?`

	if domain != "" {
		baseQuery += ` AND f.domain = ?`
		baseQuery += ` ORDER BY rank LIMIT ?`
		rows, err = d.db.Query(baseQuery, ftsQuery, domain, limit)
	} else {
		baseQuery += ` ORDER BY rank LIMIT ?`
		rows, err = d.db.Query(baseQuery, ftsQuery, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var crawledAt sql.NullTime
		if err := rows.Scan(&r.ID, &r.URL, &r.Domain, &r.Title, &crawledAt, &r.Snippet); err != nil {
			return nil, err
		}
		if crawledAt.Valid {
			r.CrawledAt = crawledAt.Time
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// quoteFTSQuery wraps each word in double quotes so FTS5 operators
// (AND, OR, NOT, NEAR, *, etc.) are treated as literal search terms.
func quoteFTSQuery(query string) string {
	words := strings.Fields(query)
	for i, w := range words {
		escaped := strings.ReplaceAll(w, `"`, `""`)
		words[i] = `"` + escaped + `"`
	}
	return strings.Join(words, " ")
}

// DomainStats holds crawl statistics for a domain.
type DomainStats struct {
	Domain    string
	Count     int
	LastCrawl time.Time
}

// ListDomains returns all crawled domains with page counts.
func (d *DB) ListDomains() ([]DomainStats, error) {
	rows, err := d.db.Query(
		`SELECT domain, COUNT(*), MAX(crawled_at) FROM pages WHERE status = 'crawled' GROUP BY domain ORDER BY domain`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []DomainStats
	for rows.Next() {
		var s DomainStats
		var lastCrawl sql.NullString
		if err := rows.Scan(&s.Domain, &s.Count, &lastCrawl); err != nil {
			return nil, err
		}
		if lastCrawl.Valid {
			s.LastCrawl = parseTime(lastCrawl.String)
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// ListPages returns all crawled pages for a domain.
func (d *DB) ListPages(domain string) ([]Page, error) {
	rows, err := d.db.Query(
		`SELECT id, url, domain, title, status, depth, html_path, md_path, md_method, crawled_at, filter_reason
		 FROM pages WHERE domain = ? AND status = 'crawled' ORDER BY url`, domain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pages []Page
	for rows.Next() {
		var p Page
		var crawledAt sql.NullTime
		if err := rows.Scan(&p.ID, &p.URL, &p.Domain, &p.Title, &p.Status, &p.Depth,
			&p.HTMLPath, &p.MDPath, &p.MDMethod, &crawledAt, &p.FilterReason); err != nil {
			return nil, err
		}
		if crawledAt.Valid {
			p.CrawledAt = crawledAt.Time
		}
		pages = append(pages, p)
	}
	return pages, rows.Err()
}

// parseTime tries common time formats stored by Go's time.Time in SQLite.
func parseTime(s string) time.Time {
	// Go's time.String() format: "2006-01-02 15:04:05.999999999 +0000 UTC m=+0.000000001"
	// Strip monotonic clock suffix if present
	if idx := strings.Index(s, " m="); idx != -1 {
		s = s[:idx]
	}
	formats := []string{
		"2006-01-02 15:04:05.999999999 +0000 UTC",
		"2006-01-02 15:04:05.999999999 -0700 MST",
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05+00:00",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func (d *DB) CountPages(domain string) (int, error) {
	var count int
	var err error
	if domain != "" {
		err = d.db.QueryRow(`SELECT COUNT(*) FROM pages WHERE domain = ?`, domain).Scan(&count)
	} else {
		err = d.db.QueryRow(`SELECT COUNT(*) FROM pages`).Scan(&count)
	}
	return count, err
}
