package converter

import (
	"net/url"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	readability "github.com/go-shiori/go-readability"
)

// Convert extracts the main article content from HTML and converts it to Markdown.
func Convert(htmlContent string, pageURL string) (string, error) {
	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		return "", err
	}

	article, err := readability.FromReader(strings.NewReader(htmlContent), parsedURL)
	if err != nil {
		return "", err
	}

	converter := md.NewConverter("", true, nil)
	markdown, err := converter.ConvertString(article.Content)
	if err != nil {
		return "", err
	}

	return markdown, nil
}
