package crawler

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/bitmorse/web2md/internal/converter"
	"github.com/bitmorse/web2md/internal/llm"
	"github.com/bitmorse/web2md/internal/storage"
	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/extensions"
)

const (
	StatusCrawled  = "crawled"
	StatusFiltered = "filtered"
	StatusSkipped  = "skipped"
)

// CrawlOptions configures the crawler.
type CrawlOptions struct {
	MaxPages  int
	MaxDepth  int
	Workers   int
	MinDelay  time.Duration
	MaxDelay  time.Duration
	ConvertMD bool
	Filter    string
	SmartMD   bool
	Recrawl   bool
	Yes       bool
	DB        *storage.DB
	DataDir   string
}

// Crawl starts crawling from startURL using the provided options.
func Crawl(startURL string, opts CrawlOptions) error {
	parsed, err := url.Parse(startURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	domain := parsed.Hostname()

	// Check robots.txt and llms.txt
	if err := checkRobotsTxt(parsed, opts.Yes); err != nil {
		return err
	}
	if err := checkLLMsTxt(parsed, opts.Yes); err != nil {
		return err
	}

	// Create domain data directory
	domainDir := filepath.Join(opts.DataDir, domain)
	if err := os.MkdirAll(domainDir, 0755); err != nil {
		return fmt.Errorf("create domain dir: %w", err)
	}

	var pageCount int32

	c := colly.NewCollector(
		colly.AllowedDomains(domain),
		colly.MaxDepth(opts.MaxDepth),
	)

	extensions.RandomUserAgent(c)

	if err := c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: opts.Workers,
		RandomDelay: opts.MaxDelay - opts.MinDelay,
		Delay:       opts.MinDelay,
	}); err != nil {
		return fmt.Errorf("set limit rule: %w", err)
	}

	if proxyURL := os.Getenv("PROXY_BASE_URL"); proxyURL != "" {
		transport := &http.Transport{}
		c.WithTransport(&proxyTransport{
			base:     transport,
			proxyURL: proxyURL,
		})
	}

	// Handle Ctrl+C — set a flag so the crawler stops accepting new pages
	var stopped int32
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nInterrupted. Finishing in-flight requests...")
		atomic.StoreInt32(&stopped, 1)
	}()

	// Track normalized URLs to avoid crawling duplicates with different query params
	visited := make(map[string]bool)
	var visitedMu sync.Mutex

	// Abort requests for already-crawled pages before the HTTP request is made
	if !opts.Recrawl {
		c.OnRequest(func(r *colly.Request) {
			existing, _ := opts.DB.GetPage(r.URL.String())
			if existing != nil && existing.Status == StatusCrawled {
				fmt.Printf("  skipping %s (already crawled, use --recrawl to re-crawl)\n", r.URL)
				r.Abort()
			}
		})
	}

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		if atomic.LoadInt32(&stopped) != 0 || int(atomic.LoadInt32(&pageCount)) >= opts.MaxPages {
			return
		}
		link := e.Request.AbsoluteURL(e.Attr("href"))
		if link == "" || shouldSkipURL(link) {
			return
		}
		norm := normalizeURL(link)
		visitedMu.Lock()
		seen := visited[norm]
		if !seen {
			visited[norm] = true
		}
		visitedMu.Unlock()
		if seen {
			return
		}
		_ = e.Request.Visit(norm)
	})

	c.OnResponse(func(r *colly.Response) {
		if atomic.LoadInt32(&stopped) != 0 || int(atomic.LoadInt32(&pageCount)) >= opts.MaxPages {
			return
		}

		// Only process HTML responses
		contentType := strings.ToLower(r.Headers.Get("Content-Type"))
		if !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "application/xhtml") {
			return
		}

		pageURL := r.Request.URL.String()
		start := time.Now()

		// Atomically claim a slot before doing work
		count := atomic.AddInt32(&pageCount, 1)
		if int(count) > opts.MaxPages {
			atomic.AddInt32(&pageCount, -1)
			return
		}

		htmlContent := string(r.Body)
		title := extractTitle(htmlContent)

		// Determine file path
		filePath := urlToFilePath(r.Request.URL)
		htmlFilePath := filepath.Join(domainDir, filePath)
		if err := os.MkdirAll(filepath.Dir(htmlFilePath), 0755); err != nil {
			fmt.Printf("  error creating dir: %v\n", err)
			atomic.AddInt32(&pageCount, -1)
			return
		}

		// Save HTML
		if err := os.WriteFile(htmlFilePath, r.Body, 0644); err != nil {
			fmt.Printf("  error saving HTML: %v\n", err)
			atomic.AddInt32(&pageCount, -1)
			return
		}
		status := StatusCrawled
		filterReason := ""

		page := &storage.Page{
			URL:       pageURL,
			Domain:    domain,
			Title:     title,
			Status:    status,
			Depth:     r.Request.Depth,
			HTMLPath:  htmlFilePath,
			CrawledAt: time.Now(),
		}

		// Filter with LLM if requested
		if opts.Filter != "" {
			matches, err := llm.FilterPage(pageURL, title, "", opts.Filter)
			if err != nil {
				fmt.Printf("  filter error: %v\n", err)
			} else if !matches {
				status = StatusFiltered
				filterReason = "did not match filter: " + opts.Filter
				page.Status = status
				page.FilterReason = filterReason
			}
		}

		// Convert to markdown
		mdFilePath := ""
		mdMethod := ""
		if opts.ConvertMD && status == StatusCrawled {
			var mdContent string
			var convErr error

			// Always try readability first
			mdContent, convErr = converter.Convert(htmlContent, pageURL)
			mdMethod = "readability"

			// With --smart-md, fall back to LLM if readability extracted too little
			if opts.SmartMD && (convErr != nil || len(strings.TrimSpace(mdContent)) < 100) {
				llmContent, llmErr := llm.ConvertToMarkdown(htmlContent)
				if llmErr == nil {
					mdContent = llmContent
					mdMethod = "llm"
					convErr = nil
				} else if convErr != nil {
					convErr = llmErr
				}
			}

			if convErr != nil {
				fmt.Printf("  markdown conversion error: %v\n", convErr)
			} else {
				mdFilePath = strings.TrimSuffix(htmlFilePath, ".html") + ".md"
				if err := os.WriteFile(mdFilePath, []byte(mdContent), 0644); err != nil {
					fmt.Printf("  error saving markdown: %v\n", err)
				} else {
					page.MDPath = mdFilePath
					page.MDMethod = mdMethod
				}
			}
		}

		if err := opts.DB.UpsertPage(page); err != nil {
			fmt.Printf("  db upsert error: %v\n", err)
		} else {
			// Index in FTS
			body := htmlContent
			if mdFilePath != "" {
				if mdBytes, err := os.ReadFile(mdFilePath); err == nil {
					body = string(mdBytes)
				}
			}
			updated, _ := opts.DB.GetPage(pageURL)
			if updated != nil {
				_ = opts.DB.InsertFTS(updated.ID, pageURL, domain, title, body)
			}
		}

		duration := time.Since(start).Milliseconds()
		fmt.Printf("[%d/%d] %s  %s  %dms\n", count, opts.MaxPages, pageURL, status, duration)
	})

	// Retry with exponential backoff (max 3 attempts)
	const maxRetries = 3
	retryCount := make(map[string]int)
	var retryMu sync.Mutex

	c.OnError(func(r *colly.Response, err error) {
		u := r.Request.URL.String()
		retryMu.Lock()
		count := retryCount[u]
		retryCount[u] = count + 1
		retryMu.Unlock()

		if count < maxRetries && r.StatusCode >= 500 {
			backoff := time.Duration(1<<uint(count)) * time.Second
			fmt.Printf("  retrying %s in %v (attempt %d/%d): %v\n", u, backoff, count+1, maxRetries, err)
			time.Sleep(backoff)
			_ = r.Request.Retry()
		} else {
			fmt.Printf("  error fetching %s: %v\n", u, err)
		}
	})

	if err := c.Visit(startURL); err != nil {
		return err
	}
	c.Wait()
	signal.Stop(sigChan)
	return nil
}

// urlToFilePath converts a URL path to a local file path.
func urlToFilePath(u *url.URL) string {
	p := u.Path
	if p == "" || p == "/" {
		return "index.html"
	}
	p = strings.TrimPrefix(p, "/")
	// Sanitize: clean the path and reject traversal attempts
	p = filepath.Clean(p)
	if p == "." || strings.HasPrefix(p, "..") || filepath.IsAbs(p) {
		return "index.html"
	}
	// Trailing slash means directory index (check original before Clean)
	if strings.HasSuffix(u.Path, "/") {
		return p + "/index.html"
	}
	// Only add .html if there's no existing file extension
	if ext := filepath.Ext(p); ext == "" {
		p = p + ".html"
	}
	return p
}

// extractTitle extracts the <title> tag content from HTML.
func extractTitle(html string) string {
	lower := strings.ToLower(html)
	start := strings.Index(lower, "<title>")
	if start == -1 {
		return ""
	}
	start += len("<title>")
	end := strings.Index(lower[start:], "</title>")
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(html[start : start+end])
}

// skipPrefixes are URL path prefixes that should never be crawled.
var skipPrefixes = []string{
	"/cdn-cgi/",
}

// skipExtensions are file extensions that are not HTML pages.
var skipExtensions = map[string]bool{
	".pdf": true, ".zip": true, ".tar": true, ".gz": true, ".rar": true,
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".svg": true, ".webp": true, ".ico": true,
	".css": true, ".js": true, ".woff": true, ".woff2": true, ".ttf": true, ".eot": true,
	".mp3": true, ".mp4": true, ".avi": true, ".mov": true, ".wmv": true,
	".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".ppt": true, ".pptx": true,
	".xml": true, ".json": true, ".csv": true,
}

// shouldSkipURL returns true for URLs that should not be crawled.
func shouldSkipURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	if strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(lower, "tel:") || strings.HasPrefix(lower, "data:") {
		return true
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return true
	}
	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(u.Path, prefix) {
			return true
		}
	}
	ext := strings.ToLower(filepath.Ext(u.Path))
	if ext != "" && skipExtensions[ext] {
		return true
	}
	return false
}

// normalizeURL strips query params, fragments, and trailing slashes for dedup.
func normalizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.RawQuery = ""
	u.Fragment = ""
	// Strip trailing slash (except for root path)
	if u.Path != "/" {
		u.Path = strings.TrimRight(u.Path, "/")
	}
	return u.String()
}

// proxyTransport rewrites the request URL to go through a proxy service
// (e.g. crawlbase) while preserving colly's original URL for dedup/filtering.
type proxyTransport struct {
	base     http.RoundTripper
	proxyURL string
}

func (t *proxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	orig := req.URL.String()
	separator := "&"
	if !strings.Contains(t.proxyURL, "?") {
		separator = "?"
	}
	proxied, err := url.Parse(t.proxyURL + separator + "url=" + url.QueryEscape(orig))
	if err != nil {
		return nil, err
	}
	req = req.Clone(req.Context())
	req.URL = proxied
	req.Host = proxied.Host
	return t.base.RoundTrip(req)
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

func checkRobotsTxt(u *url.URL, autoYes bool) error {
	robotsURL := fmt.Sprintf("%s://%s/robots.txt", u.Scheme, u.Host)
	resp, err := httpClient.Get(robotsURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	content := string(body)
	// Check for blanket disallow (Disallow: / followed by end-of-line or EOF)
	hasBlanketDisallow := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "Disallow: /" {
			hasBlanketDisallow = true
			break
		}
	}
	if hasBlanketDisallow {
		fmt.Printf("robots.txt at %s contains 'Disallow: /'\n", robotsURL)
		if autoYes {
			fmt.Println("Continuing (--yes)")
		} else {
			fmt.Print("The site may restrict crawling. Continue anyway? [y/N]: ")
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				return fmt.Errorf("crawling aborted by user due to robots.txt")
			}
		}
	}
	return nil
}

func checkLLMsTxt(u *url.URL, autoYes bool) error {
	llmsURL := fmt.Sprintf("%s://%s/llms.txt", u.Scheme, u.Host)
	resp, err := httpClient.Get(llmsURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	fmt.Printf("llms.txt found at %s:\n%s\n", llmsURL, string(body))
	if autoYes {
		fmt.Println("Continuing (--yes)")
	} else {
		fmt.Print("Continue crawling? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			return fmt.Errorf("crawling aborted by user due to llms.txt")
		}
	}
	return nil
}
