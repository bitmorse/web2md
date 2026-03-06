# CLAUDE.md

## Project overview

web2md is a Go CLI tool that crawls websites, persists pages as HTML, converts to Markdown, downloads PDFs and images, and provides FTS5-backed full-text search. Built with colly, cobra, SQLite (modernc), go-readability, and html-to-markdown.

## Project structure

```
main.go                          # Entry point, sets version/commit/date
cmd/root.go                      # Root cobra command (crawl)
cmd/search.go                    # Search subcommand
cmd/list.go                      # List crawled domains/pages
cmd/sitemap.go                   # Fetch and display sitemap.xml
internal/crawler/crawler.go      # Colly-based BFS crawler
internal/converter/converter.go  # Readability + html-to-markdown
internal/storage/storage.go      # SQLite + FTS5
internal/llm/llm.go              # OpenAI-compatible API client
```

## Development methodology

Use **red/green TDD**:

1. **Red** - Write a failing test first that defines the expected behavior
2. **Green** - Write the minimum code to make the test pass
3. **Refactor** - Clean up while keeping tests green

Always run `go test ./...` before committing. All new functionality must have tests.

## Build, test, and install

```bash
go test ./...                              # run tests
make build                                 # build binary
make install                               # install to /usr/local/bin (needs sudo)
make install INSTALL_DIR=~/.local/bin      # install to user dir
make uninstall                             # remove from /usr/local/bin
make build-all                             # cross-compile all platforms
```

## Key conventions

- Markdown conversion is ON by default (use --no-md to skip)
- PDFs and images are downloaded alongside HTML pages
- Subdomains are allowed during crawling (e.g. www.example.com, docs.example.com)
- Storage uses `~/.web2md/` (db.sqlite + data/{domain}/)
- Use `ON CONFLICT ... DO UPDATE` for upserts (not `INSERT OR REPLACE`) to preserve rowids
- FTS5 queries must be quoted with `quoteFTSQuery()` to prevent operator injection
- URL paths must be sanitized in `urlToFilePath()` to prevent directory traversal
- URLs are normalized (query params/fragments stripped) to prevent duplicate crawls
- HTTP requests to external services must have timeouts
- Signal handling uses atomic flags, not `os.Exit()`
- Version info set via ldflags: `-X main.version -X main.commit -X main.date`
