package cmd

import (
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
	convertMD bool
	filter    string
	smartMD   bool
)

var rootCmd = &cobra.Command{
	Use:   "web2md [url]",
	Short: "Crawl websites and convert them to Markdown",
	Long: `web2md crawls websites, saves HTML pages, and optionally converts them to Markdown.
Run with a URL to start crawling, or use subcommands for other operations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}

		startURL := args[0]
		if !strings.HasPrefix(startURL, "http://") && !strings.HasPrefix(startURL, "https://") {
			return fmt.Errorf("URL must start with http:// or https://")
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
			ConvertMD: convertMD,
			Filter:    filter,
			SmartMD:   smartMD,
			DB:        db,
			DataDir:   dataDir,
		}

		return crawler.Crawl(startURL, opts)
	},
}

func SetVersion(v string) {
	rootCmd.Version = v
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
	rootCmd.Flags().BoolVar(&convertMD, "convert-md", false, "Convert pages to Markdown")
	rootCmd.Flags().StringVar(&filter, "filter", "", "LLM filter description to select relevant pages")
	rootCmd.Flags().BoolVar(&smartMD, "smart-md", false, "Use LLM for Markdown conversion instead of readability")

	rootCmd.AddCommand(searchCmd)
}
