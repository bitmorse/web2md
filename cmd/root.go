package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitmorse/web2md/internal/crawler"
	"github.com/bitmorse/web2md/internal/storage"
	"github.com/spf13/cobra"
)

var (
	maxPages  int
	maxDepth  int
	workers   int
	minDelay  time.Duration
	maxDelay  time.Duration
	noMD    bool
	filter  string
	smartMD bool
	recrawl bool
	urlFile string
	yes     bool
)

var rootCmd = &cobra.Command{
	Use:   "web2md [url]",
	Short: "Crawl websites and convert them to Markdown",
	Long: `web2md crawls websites, saves HTML pages, and optionally converts them to Markdown.
Run with a URL to start crawling, or use subcommands for other operations.`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Collect URLs from args and/or --url-file
		var urls []string
		for _, arg := range args {
			if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
				urls = append(urls, arg)
			} else {
				return fmt.Errorf("URL must start with http:// or https://")
			}
		}
		if urlFile != "" {
			fileURLs, err := parseURLFile(urlFile)
			if err != nil {
				return fmt.Errorf("read url file: %w", err)
			}
			urls = append(urls, fileURLs...)
		}
		if len(urls) == 0 {
			return cmd.Help()
		}

		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("get home dir: %w", err)
		}
		baseDir := filepath.Join(homeDir, ".web2md")
		dataDir := filepath.Join(baseDir, "data")
		dbPath := filepath.Join(baseDir, "db.sqlite")

		db, err := storage.New(dbPath)
		if err != nil {
			return fmt.Errorf("open storage: %w", err)
		}
		defer db.Close()

		opts := crawler.CrawlOptions{
			MaxPages:  maxPages,
			MaxDepth:  maxDepth,
			Workers:   workers,
			MinDelay:  minDelay,
			MaxDelay:  maxDelay,
			ConvertMD: !noMD,
			Filter:    filter,
			SmartMD:   smartMD,
			Recrawl:   recrawl,
			Yes:       yes,
			DB:        db,
			DataDir:   dataDir,
		}

		for _, u := range urls {
			fmt.Printf("=== Crawling %s ===\n", u)
			if err := crawler.Crawl(u, opts); err != nil {
				fmt.Printf("  error crawling %s: %v\n", u, err)
			}
		}
		return nil
	},
}

var (
	buildVersion = "dev"
	buildCommit  = "unknown"
	buildDate    = "unknown"
)

func SetVersionInfo(version, commit, date string) {
	buildVersion = version
	buildCommit = commit
	buildDate = date
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("web2md version %s\n", buildVersion)
		fmt.Printf("Commit: %s\n", buildCommit)
		fmt.Printf("Built: %s\n", buildDate)
	},
}

func parseURLFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var urls []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "http://") && !strings.HasPrefix(line, "https://") {
			return nil, fmt.Errorf("invalid URL in file: %s", line)
		}
		urls = append(urls, line)
	}
	return urls, scanner.Err()
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().IntVar(&maxPages, "max-pages", 100, "Maximum number of pages to crawl")
	rootCmd.Flags().IntVar(&maxDepth, "max-depth", 5, "Maximum crawl depth")
	rootCmd.Flags().IntVar(&workers, "workers", 2, "Number of concurrent workers")
	rootCmd.Flags().DurationVar(&minDelay, "min-delay", 1*time.Second, "Minimum delay between requests")
	rootCmd.Flags().DurationVar(&maxDelay, "max-delay", 3*time.Second, "Maximum delay between requests")
	rootCmd.Flags().BoolVar(&noMD, "no-md", false, "Skip Markdown conversion (save HTML only)")
	rootCmd.Flags().StringVar(&filter, "filter", "", "LLM filter description to select relevant pages")
	rootCmd.Flags().BoolVar(&smartMD, "smart-md", false, "Use LLM for Markdown conversion instead of readability")
	rootCmd.Flags().BoolVar(&recrawl, "recrawl", false, "Re-crawl pages that were already crawled")
	rootCmd.Flags().StringVar(&urlFile, "url-file", "", "Path to text file with URLs (one per line)")
	rootCmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts (robots.txt, llms.txt)")

	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(versionCmd)
}
