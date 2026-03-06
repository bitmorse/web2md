package crawler

import (
	"net/http"
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

func TestProxyTransportURL(t *testing.T) {
	tests := []struct {
		name     string
		proxyURL string
		targetURL string
		wantContains string
		wantNotContains string
	}{
		{
			name:            "proxy URL ending with url=",
			proxyURL:        "https://api.crawlbase.com/?token=abc&url=",
			targetURL:       "https://example.com/page",
			wantContains:    "token=abc&url=https%3A%2F%2Fexample.com%2Fpage",
			wantNotContains: "url=&url=",
		},
		{
			name:         "proxy URL without url param",
			proxyURL:     "https://proxy.example.com",
			targetURL:    "https://example.com/page",
			wantContains: "proxy.example.com?url=https%3A%2F%2Fexample.com%2Fpage",
		},
		{
			name:         "proxy URL with query but no url=",
			proxyURL:     "https://proxy.example.com?token=abc",
			targetURL:    "https://example.com/page",
			wantContains: "token=abc&url=https%3A%2F%2Fexample.com%2Fpage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedURL string
			transport := &proxyTransport{
				base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					capturedURL = req.URL.String()
					return &http.Response{
						StatusCode: 200,
						Request:    req,
					}, nil
				}),
				proxyURL: tt.proxyURL,
			}

			origReq, _ := http.NewRequest("GET", tt.targetURL, nil)
			resp, err := transport.RoundTrip(origReq)
			if err != nil {
				t.Fatalf("RoundTrip error: %v", err)
			}

			if tt.wantContains != "" && !contains(capturedURL, tt.wantContains) {
				t.Errorf("proxy URL %q does not contain %q", capturedURL, tt.wantContains)
			}
			if tt.wantNotContains != "" && contains(capturedURL, tt.wantNotContains) {
				t.Errorf("proxy URL %q should not contain %q", capturedURL, tt.wantNotContains)
			}

			// Response should preserve the original URL for colly
			if resp.Request.URL.String() != tt.targetURL {
				t.Errorf("resp.Request.URL = %q, want original %q", resp.Request.URL.String(), tt.targetURL)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
