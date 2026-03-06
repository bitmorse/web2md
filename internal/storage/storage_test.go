package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewDB(t *testing.T) {
	dir := t.TempDir()
	db, err := New(filepath.Join(dir, "test.sqlite"))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer db.Close()
}

func TestUpsertAndGetPage(t *testing.T) {
	dir := t.TempDir()
	db, err := New(filepath.Join(dir, "test.sqlite"))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer db.Close()

	page := &Page{
		URL:       "https://example.com/page",
		Domain:    "example.com",
		Title:     "Test Page",
		Status:    "crawled",
		Depth:     1,
		HTMLPath:  "/tmp/page.html",
		CrawledAt: time.Now().UTC().Truncate(time.Second),
	}

	if err := db.UpsertPage(page); err != nil {
		t.Fatalf("UpsertPage() error: %v", err)
	}

	got, err := db.GetPage(page.URL)
	if err != nil {
		t.Fatalf("GetPage() error: %v", err)
	}
	if got == nil {
		t.Fatal("GetPage() returned nil")
	}
	if got.URL != page.URL {
		t.Errorf("URL = %q, want %q", got.URL, page.URL)
	}
	if got.Title != page.Title {
		t.Errorf("Title = %q, want %q", got.Title, page.Title)
	}
	if got.Status != page.Status {
		t.Errorf("Status = %q, want %q", got.Status, page.Status)
	}
}

func TestGetPageNotFound(t *testing.T) {
	dir := t.TempDir()
	db, err := New(filepath.Join(dir, "test.sqlite"))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer db.Close()

	got, err := db.GetPage("https://nonexistent.com/")
	if err != nil {
		t.Fatalf("GetPage() error: %v", err)
	}
	if got != nil {
		t.Errorf("GetPage() = %v, want nil", got)
	}
}

func TestUpsertPageUpdates(t *testing.T) {
	dir := t.TempDir()
	db, err := New(filepath.Join(dir, "test.sqlite"))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer db.Close()

	page := &Page{
		URL:    "https://example.com/",
		Domain: "example.com",
		Status: "crawled",
	}
	if err := db.UpsertPage(page); err != nil {
		t.Fatalf("UpsertPage() error: %v", err)
	}

	page.Status = "error"
	if err := db.UpsertPage(page); err != nil {
		t.Fatalf("UpsertPage() update error: %v", err)
	}

	got, _ := db.GetPage(page.URL)
	if got.Status != "error" {
		t.Errorf("Status after update = %q, want %q", got.Status, "error")
	}
}

func TestCountPages(t *testing.T) {
	dir := t.TempDir()
	db, err := New(filepath.Join(dir, "test.sqlite"))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer db.Close()

	pages := []Page{
		{URL: "https://a.com/1", Domain: "a.com", Status: "crawled"},
		{URL: "https://a.com/2", Domain: "a.com", Status: "crawled"},
		{URL: "https://b.com/1", Domain: "b.com", Status: "crawled"},
	}
	for _, p := range pages {
		p := p
		if err := db.UpsertPage(&p); err != nil {
			t.Fatalf("UpsertPage() error: %v", err)
		}
	}

	total, err := db.CountPages("")
	if err != nil {
		t.Fatalf("CountPages() error: %v", err)
	}
	if total != 3 {
		t.Errorf("CountPages(\"\") = %d, want 3", total)
	}

	aCount, err := db.CountPages("a.com")
	if err != nil {
		t.Fatalf("CountPages() error: %v", err)
	}
	if aCount != 2 {
		t.Errorf("CountPages(\"a.com\") = %d, want 2", aCount)
	}
}

func TestInsertFTSAndSearch(t *testing.T) {
	dir := t.TempDir()
	db, err := New(filepath.Join(dir, "test.sqlite"))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer db.Close()

	page := &Page{
		URL:    "https://example.com/about",
		Domain: "example.com",
		Title:  "About Us",
		Status: "crawled",
	}
	if err := db.UpsertPage(page); err != nil {
		t.Fatalf("UpsertPage() error: %v", err)
	}
	got, _ := db.GetPage(page.URL)

	if err := db.InsertFTS(got.ID, got.URL, got.Domain, got.Title, "We make great widgets and gadgets"); err != nil {
		t.Fatalf("InsertFTS() error: %v", err)
	}

	results, err := db.Search("widgets", "", 10)
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search() returned %d results, want 1", len(results))
	}
	if results[0].URL != page.URL {
		t.Errorf("result URL = %q, want %q", results[0].URL, page.URL)
	}
}

func TestSearchNoResults(t *testing.T) {
	dir := t.TempDir()
	db, err := New(filepath.Join(dir, "test.sqlite"))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer db.Close()

	results, err := db.Search("nonexistent", "", 10)
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Search() returned %d results, want 0", len(results))
	}
}

func TestSearchDomainFilter(t *testing.T) {
	dir := t.TempDir()
	db, err := New(filepath.Join(dir, "test.sqlite"))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer db.Close()

	pages := []struct {
		url    string
		domain string
		body   string
	}{
		{"https://a.com/page", "a.com", "starch residue analysis"},
		{"https://b.com/page", "b.com", "starch residue testing"},
	}
	for _, p := range pages {
		page := &Page{URL: p.url, Domain: p.domain, Status: "crawled"}
		if err := db.UpsertPage(page); err != nil {
			t.Fatalf("UpsertPage() error: %v", err)
		}
		got, _ := db.GetPage(p.url)
		if err := db.InsertFTS(got.ID, got.URL, got.Domain, "", p.body); err != nil {
			t.Fatalf("InsertFTS() error: %v", err)
		}
	}

	results, err := db.Search("starch", "a.com", 10)
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search() with domain filter returned %d results, want 1", len(results))
	}
	if results[0].Domain != "a.com" {
		t.Errorf("result domain = %q, want a.com", results[0].Domain)
	}
}

func TestDBPath(t *testing.T) {
	dir := t.TempDir()
	nestedPath := filepath.Join(dir, "nested", "dir", "test.sqlite")
	db, err := New(nestedPath)
	if err != nil {
		t.Fatalf("New() with nested path error: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(nestedPath); os.IsNotExist(err) {
		t.Error("db file was not created")
	}
}
