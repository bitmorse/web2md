package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitmorse/web2md/internal/storage"
	"github.com/spf13/cobra"
)

const boxWidth = 56 // content width inside borders

var (
	searchDomain string
	searchLimit  int
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search crawled pages",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")

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

		results, err := db.Search(query, searchDomain, searchLimit)
		if err != nil {
			return fmt.Errorf("search: %w", err)
		}

		total, err := db.CountPages(searchDomain)
		if err != nil {
			return fmt.Errorf("count pages: %w", err)
		}

		if len(results) == 0 {
			fmt.Printf("No results for %q\n", query)
			return nil
		}

		printBoxTop()
		for i, r := range results {
			title := r.Title
			if title == "" {
				title = "(no title)"
			}
			snippet := highlightKeywords(r.Snippet, strings.Fields(query))
			date := r.CrawledAt.Format("2006-01-02")

			printBoxLine(fmt.Sprintf("[%d] %s", i+1, title))
			printBoxLine(r.URL)
			// Wrap snippet across multiple lines if needed
			for _, line := range wrapText(snippet, boxWidth) {
				printBoxLine(line)
			}
			printBoxLine("Crawled: " + date)

			if i < len(results)-1 {
				printBoxMiddle()
			}
		}
		printBoxBottom()

		domainStr := ""
		if searchDomain != "" {
			domainStr = fmt.Sprintf(" (%s)", searchDomain)
		}
		fmt.Printf("%d results for %q across %d pages%s\n", len(results), query, total, domainStr)
		return nil
	},
}

func init() {
	searchCmd.Flags().StringVarP(&searchDomain, "domain", "d", "", "Filter results by domain")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 20, "Maximum number of results")
}

func printBoxTop() {
	fmt.Printf("┌%s┐\n", strings.Repeat("─", boxWidth+2))
}

func printBoxMiddle() {
	fmt.Printf("├%s┤\n", strings.Repeat("─", boxWidth+2))
}

func printBoxBottom() {
	fmt.Printf("└%s┘\n", strings.Repeat("─", boxWidth+2))
}

func printBoxLine(text string) {
	// Truncate if too long, pad if too short
	runes := []rune(text)
	if len(runes) > boxWidth {
		runes = runes[:boxWidth-3]
		runes = append(runes, []rune("...")...)
	}
	padded := string(runes)
	padding := boxWidth - len([]rune(padded))
	fmt.Printf("│ %s%s │\n", padded, strings.Repeat(" ", padding))
}

// highlightKeywords converts words matching query terms to UPPERCASE.
func highlightKeywords(text string, keywords []string) string {
	words := strings.Fields(text)
	for i, word := range words {
		clean := strings.ToLower(strings.Trim(word, ".,!?;:\"'()[]{}"))
		for _, kw := range keywords {
			if clean == strings.ToLower(kw) {
				words[i] = strings.ToUpper(word)
				break
			}
		}
	}
	return strings.Join(words, " ")
}

// wrapText wraps text to the given width.
func wrapText(text string, width int) []string {
	if text == "" {
		return []string{""}
	}
	var lines []string
	words := strings.Fields(text)
	current := ""
	for _, w := range words {
		if current == "" {
			current = w
		} else if len([]rune(current))+1+len([]rune(w)) <= width {
			current += " " + w
		} else {
			lines = append(lines, current)
			current = w
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
