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
	if err := checkRobotsTxt(parsed); err != nil {
		return err
	}
	if err := checkLLMsTxt(parsed); err != nil {
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
		c.OnRequest(func(r *colly.Request) {
			orig := r.URL.String()
			separator := "&"
			if !strings.Contains(proxyURL, "?") {
				separator = "?"
			}
			newURL := proxyURL + separator + "url=" + orig
			parsed, err := url.Parse(newURL)
			if err == nil {
				r.URL = parsed
			}
		})
	}

	// Handle Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nInterrupted. Stopping crawler...")
		c.Wait()
		os.Exit(0)
	}()

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		if int(atomic.LoadInt32(&pageCount)) >= opts.MaxPages {
			return
		}
		link := e.Attr("href")
		_ = e.Request.Visit(link)
	})

	c.OnResponse(func(r *colly.Response) {
		if int(atomic.LoadInt32(&pageCount)) >= opts.MaxPages {
			return
		}

		pageURL := r.Request.URL.String()
		start := time.Now()

		// Resume: skip already-crawled pages
		existing, _ := opts.DB.GetPage(pageURL)
		if existing != nil && existing.Status == StatusCrawled {
			fmt.Printf("[%d/%d] %s  %s (already crawled)\n",
				atomic.LoadInt32(&pageCount), opts.MaxPages, pageURL, StatusSkipped)
			return
		}

		htmlContent := string(r.Body)
		title := extractTitle(htmlContent)

		// Determine file path
		filePath := urlToFilePath(r.Request.URL)
		htmlFilePath := filepath.Join(domainDir, filePath)
		if err := os.MkdirAll(filepath.Dir(htmlFilePath), 0755); err != nil {
			fmt.Printf("  error creating dir: %v\n", err)
			return
		}

		// Save HTML
		if err := os.WriteFile(htmlFilePath, r.Body, 0644); err != nil {
			fmt.Printf("  error saving HTML: %v\n", err)
			return
		}

		count := atomic.AddInt32(&pageCount, 1)
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

			if opts.SmartMD {
				mdContent, convErr = llm.ConvertToMarkdown(htmlContent)
				mdMethod = "llm"
			} else {
				mdContent, convErr = converter.Convert(htmlContent, pageURL)
				mdMethod = "readability"
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

	c.OnError(func(r *colly.Response, err error) {
		fmt.Printf("  error fetching %s: %v\n", r.Request.URL, err)
	})

	return c.Visit(startURL)
}

// urlToFilePath converts a URL path to a local file path.
func urlToFilePath(u *url.URL) string {
	p := u.Path
	if p == "" || p == "/" {
		return "index.html"
	}
	p = strings.TrimPrefix(p, "/")
	// Trailing slash means directory index
	if strings.HasSuffix(p, "/") {
		return p + "index.html"
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

func checkRobotsTxt(u *url.URL) error {
	robotsURL := fmt.Sprintf("%s://%s/robots.txt", u.Scheme, u.Host)
	resp, err := http.Get(robotsURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	content := string(body)
	if strings.Contains(content, "Disallow: /") {
		fmt.Printf("robots.txt at %s contains 'Disallow: /'\n", robotsURL)
		fmt.Print("The site may restrict crawling. Continue anyway? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			return fmt.Errorf("crawling aborted by user due to robots.txt")
		}
	}
	return nil
}

func checkLLMsTxt(u *url.URL) error {
	llmsURL := fmt.Sprintf("%s://%s/llms.txt", u.Scheme, u.Host)
	resp, err := http.Get(llmsURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	fmt.Printf("llms.txt found at %s:\n%s\n", llmsURL, string(body))
	fmt.Print("Continue crawling? [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		return fmt.Errorf("crawling aborted by user due to llms.txt")
	}
	return nil
}
