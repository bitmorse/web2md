package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bitmorse/web2md/internal/storage"
	"github.com/spf13/cobra"
)

var listDomain string

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List crawled domains and pages",
	RunE: func(cmd *cobra.Command, args []string) error {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("get home dir: %w", err)
		}
		dbPath := filepath.Join(homeDir, ".web2md", "db.sqlite")

		db, err := storage.New(dbPath)
		if err != nil {
			return fmt.Errorf("open storage: %w", err)
		}
		defer db.Close()

		if listDomain != "" {
			pages, err := db.ListPages(listDomain)
			if err != nil {
				return fmt.Errorf("list pages: %w", err)
			}
			if len(pages) == 0 {
				fmt.Printf("No crawled pages for %s\n", listDomain)
				return nil
			}
			fmt.Printf("%s (%d pages)\n\n", listDomain, len(pages))
			for _, p := range pages {
				title := p.Title
				if title == "" {
					title = "(no title)"
				}
				md := ""
				if p.MDPath != "" {
					md = " [md]"
				}
				fmt.Printf("  %s  %s%s\n", p.URL, title, md)
			}
			return nil
		}

		stats, err := db.ListDomains()
		if err != nil {
			return fmt.Errorf("list domains: %w", err)
		}
		if len(stats) == 0 {
			fmt.Println("No crawled domains yet.")
			return nil
		}

		dataDir := filepath.Join(filepath.Dir(dbPath), "data")

		fmt.Printf("%-40s %6s %9s  %s\n", "DOMAIN", "PAGES", "SIZE", "LAST CRAWLED")
		for _, s := range stats {
			size := dirSize(filepath.Join(dataDir, s.Domain))
			fmt.Printf("%-40s %6d %9s  %s\n", s.Domain, s.Count, formatSize(size), s.LastCrawl.Format("2006-01-02 15:04"))
		}
		return nil
	},
}

func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		size += info.Size()
		return nil
	})
	return size
}

func formatSize(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func init() {
	listCmd.Flags().StringVarP(&listDomain, "domain", "d", "", "List pages for a specific domain")
	rootCmd.AddCommand(listCmd)
}
