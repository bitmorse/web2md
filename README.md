# web2md

A CLI tool that crawls websites, saves pages as HTML, converts them to Markdown, downloads PDFs and images, and provides full-text search across crawled content.

## Quick Install

```bash
curl -s https://raw.githubusercontent.com/bitmorse/web2md/main/install.sh | bash
```

Or with Go:

```bash
go install github.com/bitmorse/web2md@latest
```

## Usage

### Crawl a website

```bash
web2md https://example.com
```

Pages are saved as HTML and converted to Markdown by default. PDFs and images found during the crawl are also downloaded.

HTML only (skip Markdown conversion):

```bash
web2md https://example.com --no-md
```

Skip confirmation prompts (robots.txt, llms.txt):

```bash
web2md https://example.com -y
```

### Batch crawl from a file

```bash
web2md --url-file sites.txt
```

Where `sites.txt` contains one URL per line (`#` comments and blank lines are ignored).

### Check a site's size before crawling

```bash
web2md sitemap https://example.com
```

Fetches `sitemap.xml` and shows the total number of URLs.

### List crawled sites

```bash
web2md list
```

```
DOMAIN                                    PAGES      SIZE  LAST CRAWLED
epfl.ch                                     100    7.6 MB  2026-03-06 18:04
octanis.ch                                   15  688.7 KB  2026-03-06 17:54
```

List pages for a specific domain:

```bash
web2md list -d octanis.ch
```

### Search crawled pages

```bash
web2md search "query"
web2md search "query" -d example.com
```

### Version

```bash
web2md version
```

### Crawl flags

| Flag | Default | Description |
|------|---------|-------------|
| `--max-pages` | 100 | Maximum number of pages to crawl |
| `--max-depth` | 5 | Maximum crawl depth |
| `--workers` | 2 | Concurrent workers |
| `--min-delay` | 1s | Minimum delay between requests |
| `--max-delay` | 3s | Maximum delay between requests |
| `--no-md` | false | Skip Markdown conversion (save HTML only) |
| `--smart-md` | false | Use readability with LLM fallback for Markdown conversion |
| `--filter` | "" | LLM-powered page filter description |
| `--recrawl` | false | Re-crawl pages that were already crawled |
| `--url-file` | "" | Path to text file with URLs (one per line) |
| `-y`, `--yes` | false | Skip confirmation prompts |

### Search flags

| Flag | Default | Description |
|------|---------|-------------|
| `-d`, `--domain` | "" | Filter results by domain |
| `--limit` | 20 | Maximum number of results |

## How it works

1. **Crawl** - BFS crawl using [colly](https://github.com/gocolly/colly), same-domain + subdomains, respects robots.txt/llms.txt
2. **Store** - HTML, PDFs, and images saved to `~/.web2md/data/{domain}/`, metadata in SQLite at `~/.web2md/db.sqlite`
3. **Convert** - Markdown conversion via [go-readability](https://github.com/go-shiori/go-readability) + [html-to-markdown](https://github.com/JohannesKaufmann/html-to-markdown) (on by default)
4. **Search** - FTS5 full-text search with BM25 ranking

### Crawl defenses

- URL normalization (strips query params/fragments to prevent duplicates)
- Skips non-page resources (CSS, JS, fonts, archives)
- Downloads valuable assets (PDFs, images)
- Filters out Cloudflare email protection links
- Content-type validation (only processes HTML for page extraction)
- Retry with exponential backoff on 5xx errors (3 attempts)
- Sitemap check before crawling with page count estimate

### Resume support

Re-running a crawl on the same domain skips already-crawled URLs. Use `--recrawl` to force re-crawling:

```bash
web2md https://example.com --recrawl
```

## Environment variables

| Variable | Description |
|----------|-------------|
| `PROXY_BASE_URL` | Proxy service URL (e.g. crawlbase) |
| `OPENAI_API_KEY` | Required for `--filter` and `--smart-md` |
| `OPENAI_BASE_URL` | Custom OpenAI-compatible API endpoint |
| `OPENAI_MODEL` | Model to use (default: `gpt-4o-mini`) |

## Building from source

```bash
git clone https://github.com/bitmorse/web2md.git
cd web2md
make build
make install
```

Cross-compile all platforms:

```bash
make build-all
```

## License

GPL-3.0
