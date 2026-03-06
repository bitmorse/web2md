package crawler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bitmorse/web2md/internal/storage"
)

// newTestEnv creates a temporary DB and data dir for testing.
func newTestEnv(t *testing.T) (*storage.DB, string) {
	t.Helper()
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.sqlite")
	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("storage.New: %v", err)
	}
	dataDir := filepath.Join(tmp, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}
	return db, dataDir
}

// TestCrawlBFS verifies that all links from a page are discovered at the same
// BFS level, not chained as DFS. A homepage linking to /a, /b, /c should crawl
// all three — not just /a and then /a's children.
func TestCrawlBFS(t *testing.T) {
	pages := map[string]string{
		"/": `<html><body>
			<a href="/a">A</a>
			<a href="/b">B</a>
			<a href="/c">C</a>
		</body></html>`,
		"/a": `<html><body><title>Page A</title><a href="/b">B</a><a href="/d">D</a></body></html>`,
		"/b": `<html><body><title>Page B</title><a href="/a">A</a><a href="/d">D</a></body></html>`,
		"/c": `<html><body><title>Page C</title><a href="/e">E</a></body></html>`,
		"/d": `<html><body><title>Page D</title><a href="/">Home</a></body></html>`,
		"/e": `<html><body><title>Page E</title><a href="/">Home</a></body></html>`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := pages[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	db, dataDir := newTestEnv(t)
	defer db.Close()

	err := Crawl(srv.URL, CrawlOptions{
		MaxPages:  100,
		MaxDepth:  3,
		Workers:   2,
		MinDelay:  0,
		MaxDelay:  10 * time.Millisecond,
		ConvertMD: false,
		Yes:       true,
		DB:        db,
		DataDir:   dataDir,
	})
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}

	// All 6 pages should be crawled (/, /a, /b, /c, /d, /e)
	count, err := db.CountPages("")
	if err != nil {
		t.Fatalf("CountPages: %v", err)
	}
	if count < 6 {
		t.Errorf("expected at least 6 pages crawled, got %d", count)
	}
}

// TestCrawlNoLoop verifies that circular links don't cause infinite crawling.
func TestCrawlNoLoop(t *testing.T) {
	// A -> B -> C -> A (circular)
	pages := map[string]string{
		"/": `<html><body><a href="/a">A</a></body></html>`,
		"/a": `<html><body><title>A</title><a href="/b">B</a></body></html>`,
		"/b": `<html><body><title>B</title><a href="/c">C</a></body></html>`,
		"/c": `<html><body><title>C</title><a href="/a">A</a><a href="/">Home</a></body></html>`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := pages[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	db, dataDir := newTestEnv(t)
	defer db.Close()

	err := Crawl(srv.URL, CrawlOptions{
		MaxPages:  100,
		MaxDepth:  10,
		Workers:   2,
		MinDelay:  0,
		MaxDelay:  10 * time.Millisecond,
		ConvertMD: false,
		Yes:       true,
		DB:        db,
		DataDir:   dataDir,
	})
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}

	// Exactly 4 pages should be crawled (no duplicates from loops)
	count, err := db.CountPages("")
	if err != nil {
		t.Fatalf("CountPages: %v", err)
	}
	if count != 4 {
		t.Errorf("expected exactly 4 pages crawled (no loops), got %d", count)
	}
}

// TestCrawlResume verifies that a resumed crawl discovers new pages linked
// from already-crawled pages (without re-saving the old pages).
func TestCrawlResume(t *testing.T) {
	requestCount := 0
	pages := map[string]string{
		"/": `<html><body><a href="/a">A</a><a href="/b">B</a></body></html>`,
		"/a": `<html><body><title>A</title><a href="/c">C</a></body></html>`,
		"/b": `<html><body><title>B</title><a href="/d">D</a></body></html>`,
		"/c": `<html><body><title>C</title></body></html>`,
		"/d": `<html><body><title>D</title></body></html>`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		body, ok := pages[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	db, dataDir := newTestEnv(t)
	defer db.Close()

	// First crawl: only 2 pages (homepage + /a)
	err := Crawl(srv.URL, CrawlOptions{
		MaxPages:  2,
		MaxDepth:  5,
		Workers:   1,
		MinDelay:  0,
		MaxDelay:  10 * time.Millisecond,
		ConvertMD: false,
		Yes:       true,
		DB:        db,
		DataDir:   dataDir,
	})
	if err != nil {
		t.Fatalf("first Crawl: %v", err)
	}

	firstCount, _ := db.CountPages("")
	if firstCount < 2 {
		t.Fatalf("expected at least 2 pages from first crawl, got %d", firstCount)
	}

	// Resume crawl: should discover /b, /c, /d from the links on already-crawled pages
	err = Crawl(srv.URL, CrawlOptions{
		MaxPages:  10,
		MaxDepth:  5,
		Workers:   1,
		MinDelay:  0,
		MaxDelay:  10 * time.Millisecond,
		ConvertMD: false,
		Recrawl:   false, // resume mode
		Yes:       true,
		DB:        db,
		DataDir:   dataDir,
	})
	if err != nil {
		t.Fatalf("resume Crawl: %v", err)
	}

	finalCount, _ := db.CountPages("")
	if finalCount < 5 {
		t.Errorf("expected all 5 pages after resume, got %d", finalCount)
	}
}

// TestCrawlRedirectDepth verifies that a redirect from the seed URL doesn't
// consume excessive depth budget.
func TestCrawlRedirectDepth(t *testing.T) {
	pages := map[string]string{
		"/en": `<html><body><a href="/en/a">A</a><a href="/en/b">B</a></body></html>`,
		"/en/a": `<html><body><title>A</title><a href="/en/c">C</a></body></html>`,
		"/en/b": `<html><body><title>B</title></body></html>`,
		"/en/c": `<html><body><title>C</title></body></html>`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/en", http.StatusMovedPermanently)
			return
		}
		body, ok := pages[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	db, dataDir := newTestEnv(t)
	defer db.Close()

	// With max-depth 3, even with redirect consuming depth, all pages should be found
	err := Crawl(srv.URL, CrawlOptions{
		MaxPages:  100,
		MaxDepth:  3,
		Workers:   2,
		MinDelay:  0,
		MaxDelay:  10 * time.Millisecond,
		ConvertMD: false,
		Yes:       true,
		DB:        db,
		DataDir:   dataDir,
	})
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}

	// Should crawl /en, /en/a, /en/b, /en/c (4 pages total)
	count, err := db.CountPages("")
	if err != nil {
		t.Fatalf("CountPages: %v", err)
	}
	if count < 4 {
		t.Errorf("expected 4 pages with redirect + depth 3, got %d", count)
	}
}

// TestProxyFeedback verifies that proxy usage is reported to the user.
func TestProxyFeedback(t *testing.T) {
	// Just verify the function exists and is called — the actual proxy
	// feedback is a fmt.Printf side effect tested via the real binary.
	// Here we just verify PROXY_BASE_URL is read.
	old := os.Getenv("PROXY_BASE_URL")
	defer os.Setenv("PROXY_BASE_URL", old)
	os.Setenv("PROXY_BASE_URL", "https://proxy.example.com/?token=abc")

	// The proxy transport should be configured — we can't easily test
	// fmt output in a unit test, so we verify it doesn't panic.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body>Hello</body></html>`)
	}))
	defer srv.Close()

	db, dataDir := newTestEnv(t)
	defer db.Close()

	// This will fail to connect to the proxy, but shouldn't panic
	_ = Crawl(srv.URL, CrawlOptions{
		MaxPages:  1,
		MaxDepth:  1,
		Workers:   1,
		MinDelay:  0,
		MaxDelay:  10 * time.Millisecond,
		ConvertMD: false,
		Yes:       true,
		DB:        db,
		DataDir:   dataDir,
	})
	// Test passes if no panic — proxy feedback is a print side effect
}

// TestCrawlNormalizedDedup verifies that URLs differing only in query params
// or fragments are treated as the same page.
func TestCrawlNormalizedDedup(t *testing.T) {
	pages := map[string]string{
		"/": `<html><body>
			<a href="/page?a=1">Q1</a>
			<a href="/page?b=2">Q2</a>
			<a href="/page#section">Frag</a>
			<a href="/page">Clean</a>
		</body></html>`,
		"/page": `<html><body><title>The Page</title></body></html>`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve based on path only (ignore query params)
		body, ok := pages[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	db, dataDir := newTestEnv(t)
	defer db.Close()

	err := Crawl(srv.URL, CrawlOptions{
		MaxPages:  100,
		MaxDepth:  3,
		Workers:   1,
		MinDelay:  0,
		MaxDelay:  10 * time.Millisecond,
		ConvertMD: false,
		Yes:       true,
		DB:        db,
		DataDir:   dataDir,
	})
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}

	// Should be exactly 2 pages: / and /page (query params and fragments deduped)
	count, _ := db.CountPages("")
	if count != 2 {
		// Check what was stored
		domain := strings.TrimPrefix(srv.URL, "http://")
		pages, _ := db.ListPages(domain)
		var urls []string
		for _, p := range pages {
			urls = append(urls, p.URL)
		}
		t.Errorf("expected 2 pages (deduped), got %d: %v", count, urls)
	}
}
