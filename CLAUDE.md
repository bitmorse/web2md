# CLAUDE.md

## Project overview

web2md is a Go CLI tool that crawls websites, persists pages as HTML, optionally converts to Markdown, and provides FTS5-backed full-text search. Built with colly, cobra, SQLite (modernc), go-readability, and html-to-markdown.

## Project structure

```
main.go                          # Entry point, sets version
cmd/root.go                      # Root cobra command (crawl)
cmd/search.go                    # Search subcommand
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

## Build and test

```bash
go build ./...
go test ./...
make build          # build binary
make build-all      # cross-compile all platforms
```

## Key conventions

- Storage uses `~/.web2md/` (db.sqlite + data/{domain}/)
- Use `ON CONFLICT ... DO UPDATE` for upserts (not `INSERT OR REPLACE`) to preserve rowids
- FTS5 queries must be quoted with `quoteFTSQuery()` to prevent operator injection
- URL paths must be sanitized in `urlToFilePath()` to prevent directory traversal
- HTTP requests to external services must have timeouts
- Signal handling uses atomic flags, not `os.Exit()`
