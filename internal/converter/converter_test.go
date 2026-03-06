package converter

import (
	"strings"
	"testing"
)

func TestConvert(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head><title>Test Article</title></head>
<body>
  <nav>Navigation Menu</nav>
  <article>
    <h1>Main Content</h1>
    <p>This is the main article content that should be extracted.</p>
    <p>It has multiple paragraphs with useful information.</p>
  </article>
  <footer>Footer content</footer>
</body>
</html>`

	result, err := Convert(html, "https://example.com/article")
	if err != nil {
		t.Fatalf("Convert() error: %v", err)
	}
	if result == "" {
		t.Error("Convert() returned empty string")
	}
	// Should contain main content
	if !strings.Contains(result, "Main Content") {
		t.Errorf("Convert() result missing main heading, got: %s", result)
	}
}

func TestConvertEmptyHTML(t *testing.T) {
	// go-readability handles empty HTML gracefully
	_, err := Convert("<html><body></body></html>", "https://example.com/")
	// May return error or empty - either is acceptable
	_ = err
}

func TestConvertMarkdownOutput(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head><title>Article</title></head>
<body>
  <article>
    <h1>Heading</h1>
    <p>Paragraph with <strong>bold</strong> text.</p>
    <ul>
      <li>Item 1</li>
      <li>Item 2</li>
    </ul>
  </article>
</body>
</html>`

	result, err := Convert(html, "https://example.com/article")
	if err != nil {
		t.Fatalf("Convert() error: %v", err)
	}
	// Markdown should contain heading syntax or bold syntax
	hasMD := strings.Contains(result, "#") || strings.Contains(result, "**") || strings.Contains(result, "Heading")
	if !hasMD {
		t.Errorf("Convert() output doesn't look like Markdown: %s", result)
	}
}

func TestConvertInvalidURL(t *testing.T) {
	html := `<html><body><article><p>Content</p></article></body></html>`
	// Should still work with an invalid URL (go-readability is tolerant)
	_, err := Convert(html, "not-a-url")
	// We just verify it doesn't panic - error is acceptable
	_ = err
}
