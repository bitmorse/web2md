package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
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
		`INSERT OR REPLACE INTO pages (url, domain, title, status, depth, html_path, md_path, md_method, crawled_at, filter_reason)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
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

	baseQuery := `SELECT f.rowid, f.url, f.domain, f.title, p.crawled_at,
		snippet(pages_fts, 3, '', '', '...', 30)
		FROM pages_fts f
		LEFT JOIN pages p ON p.url = f.url
		WHERE pages_fts MATCH ?`

	if domain != "" {
		baseQuery += ` AND f.domain = ?`
		baseQuery += ` ORDER BY rank LIMIT ?`
		rows, err = d.db.Query(baseQuery, query, domain, limit)
	} else {
		baseQuery += ` ORDER BY rank LIMIT ?`
		rows, err = d.db.Query(baseQuery, query, limit)
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
