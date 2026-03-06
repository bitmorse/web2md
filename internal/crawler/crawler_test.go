package crawler

import (
	"testing"
	"net/url"
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
