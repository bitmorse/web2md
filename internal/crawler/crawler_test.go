package crawler

import (
	"net/url"
	"testing"
)

func TestURLToFilePath(t *testing.T) {
	tests := []struct {
		rawURL string
		want   string
	}{
		{"https://example.com/", "index.html"},
		{"https://example.com", "index.html"},
		{"https://example.com/about", "about.html"},
		{"https://example.com/en/about", "en/about.html"},
		{"https://example.com/style.css", "style.css"},
		{"https://example.com/image.png", "image.png"},
		{"https://example.com/page/", "page/index.html"},
		// Path traversal attempts should be sanitized
		{"https://example.com/../../etc/passwd", "index.html"},
		{"https://example.com/../secret", "index.html"},
		{"https://example.com/a/b/../../c", "c.html"},
	}

	for _, tt := range tests {
		u, err := url.Parse(tt.rawURL)
		if err != nil {
			t.Fatalf("url.Parse(%q) error: %v", tt.rawURL, err)
		}
		got := urlToFilePath(u)
		if got != tt.want {
			t.Errorf("urlToFilePath(%q) = %q, want %q", tt.rawURL, got, tt.want)
		}
	}
}

func TestCrawlOptionsHasRecrawl(t *testing.T) {
	// Recrawl field must exist on CrawlOptions and default to false
	opts := CrawlOptions{}
	if opts.Recrawl {
		t.Fatal("Recrawl should default to false")
	}
	opts.Recrawl = true
	if !opts.Recrawl {
		t.Fatal("Recrawl should be settable to true")
	}
}

func TestShouldSkipURL(t *testing.T) {
	tests := []struct {
		rawURL string
		skip   bool
	}{
		// Normal pages — don't skip
		{"https://example.com/about", false},
		{"https://example.com/en/contact", false},
		{"https://example.com/", false},
		// Cloudflare email protection — skip
		{"https://example.com/cdn-cgi/l/email-protection", true},
		{"https://example.com/cdn-cgi/l/email-protection#abc2c5cdc4", true},
		// Other cdn-cgi paths — skip
		{"https://example.com/cdn-cgi/challenge-platform/something", true},
		// Fragment-only or javascript — skip
		{"javascript:void(0)", true},
		{"mailto:test@example.com", true},
		// Downloadable files — don't skip (PDFs, images are saved)
		{"https://example.com/file.pdf", false},
		{"https://example.com/image.jpg", false},
		{"https://example.com/image.png", false},
		// Non-useful extensions — skip
		{"https://example.com/archive.zip", true},
		{"https://example.com/style.css", true},
		{"https://example.com/script.js", true},
		// Pages with extensions — don't skip
		{"https://example.com/page.html", false},
		{"https://example.com/page.htm", false},
		{"https://example.com/page.php", false},
	}

	for _, tt := range tests {
		got := shouldSkipURL(tt.rawURL)
		if got != tt.skip {
			t.Errorf("shouldSkipURL(%q) = %v, want %v", tt.rawURL, got, tt.skip)
		}
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		rawURL string
		want   string
	}{
		// Strip query params
		{"https://example.com/contact?service=ai", "https://example.com/contact"},
		{"https://example.com/page?a=1&b=2", "https://example.com/page"},
		// Strip fragments
		{"https://example.com/about#section", "https://example.com/about"},
		// No change needed
		{"https://example.com/about", "https://example.com/about"},
		// Strip trailing slash for consistency (except root)
		{"https://example.com/about/", "https://example.com/about"},
		{"https://example.com/", "https://example.com/"},
	}

	for _, tt := range tests {
		got := normalizeURL(tt.rawURL)
		if got != tt.want {
			t.Errorf("normalizeURL(%q) = %q, want %q", tt.rawURL, got, tt.want)
		}
	}
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		html string
		want string
	}{
		{"<html><head><title>My Page</title></head><body></body></html>", "My Page"},
		{"<html><head><TITLE>Uppercase</TITLE></head></html>", "Uppercase"},
		{"<html><head></head><body>No title</body></html>", ""},
		{"<title>  Trimmed  </title>", "Trimmed"},
	}

	for _, tt := range tests {
		got := extractTitle(tt.html)
		if got != tt.want {
			t.Errorf("extractTitle(%q) = %q, want %q", tt.html, got, tt.want)
		}
	}
}
