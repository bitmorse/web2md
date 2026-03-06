package cmd

import (
	"fmt"

	"github.com/bitmorse/web2md/internal/crawler"
	"github.com/spf13/cobra"
)

var sitemapCmd = &cobra.Command{
	Use:   "sitemap <url>",
	Short: "Fetch and display a site's sitemap",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		info, err := crawler.FetchSitemap(args[0])
		if err != nil {
			return err
		}

		fmt.Printf("Sitemap: %s\n", info.URL)
		fmt.Printf("Total URLs: %d\n\n", info.Count)
		for _, u := range info.URLs {
			fmt.Println(u)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(sitemapCmd)
}
