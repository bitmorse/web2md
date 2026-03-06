# web2md

A CLI tool that crawls websites, saves pages as HTML, optionally converts them to Markdown, and provides full-text search across crawled content.

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

HTML only (skip Markdown conversion):

```bash
web2md https://example.com --no-md
```

### Search crawled pages

```bash
web2md search "query"
web2md search "query" -d example.com
```

### Flags

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

### Search flags

| Flag | Default | Description |
|------|---------|-------------|
| `-d`, `--domain` | "" | Filter results by domain |
| `--limit` | 20 | Maximum number of results |

## How it works

1. **Crawl** - BFS crawl using [colly](https://github.com/gocolly/colly), same-domain only, respects robots.txt/llms.txt
2. **Store** - HTML saved to `~/.web2md/data/{domain}/`, metadata in SQLite at `~/.web2md/db.sqlite`
3. **Convert** - Optional Markdown conversion via [go-readability](https://github.com/go-shiori/go-readability) + [html-to-markdown](https://github.com/JohannesKaufmann/html-to-markdown)
4. **Search** - FTS5 full-text search with BM25 ranking

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
```

Cross-compile all platforms:

```bash
make build-all
```

## License

GPL-3.0
