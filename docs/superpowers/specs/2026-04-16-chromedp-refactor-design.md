# Refactor HTML Scrapers to chromedp + goquery

## Problem

The tvsporedi.si website moved behind Cloudflare, which blocks requests from Colly (a Go HTTP scraping library that doesn't execute JavaScript). The scraper now returns 403 Forbidden.

## Scope

- **tvsporedi.go** — replace Colly with chromedp + goquery, preserve existing parsing logic
- **siol.go** — replace Colly with chromedp + goquery, fully implement programme scraping (currently stubbed)
- **browser.go** — new shared helper for chromedp page fetching
- **t2.go, a1.go** — untouched (JSON APIs, no browser needed)
- **schema.go, main.go** — unchanged

## Approach

Two-phase pattern for each HTML scraper:

1. **Fetch phase (chromedp):** Launch headless Chrome, navigate to URL, wait for page to render (including Cloudflare JS challenge), extract full HTML
2. **Parse phase (goquery):** Load HTML into goquery, use CSS selectors to extract data

## New file: browser.go

Shared helper to avoid duplicating chromedp boilerplate:

```go
func fetchRenderedHTML(targetURL string) (string, error)
```

- Creates a chromedp context with a 60-second timeout (enough for Cloudflare challenge)
- Navigates to the URL
- Waits for `body` to be visible
- Returns outer HTML of the document

## tvsporedi.go changes

- Remove Colly collector and detailCollector setup
- Call `fetchRenderedHTML("https://www.tvsporedi.si/spored.php")` for the main page
- Parse channel links with goquery using selector `div[id=left-navigation] a[href]`
- For each channel URL, call `fetchRenderedHTML` again and parse programmes with goquery using selector `div[class=schedule] > div[class!=clear]`
- Keep all existing helper functions: `parseTime`, `findFullPrevSibling`, `timeOffset`, `getDesc`, `processData` — goquery exposes `*html.Node` so these work unchanged

## siol.go changes

- Remove Colly collector setup
- Call `fetchRenderedHTML("https://tv-spored.siol.net/kanali")` for the channels page
- Parse channel list with goquery using selector `div[class=table-list-rows] a[href]`
- For each channel, fetch schedule pages for today + next 2 days
- Extract programme data: time, title from schedule rows
- For each programme, fetch the show detail page and extract description, cast, etc.
- Keep `getTodayDatePlusString`, `parseShowTime` helpers

## Go module changes

- Bump `go` directive to 1.21+ (chromedp requirement)
- Add `github.com/chromedp/chromedp`
- Add `github.com/PuerkitoBio/goquery` (already an indirect dep via Colly, will become direct)
- Remove `github.com/gocolly/colly` and its transitive-only deps

## What stays the same

- `schema.go` — no changes
- `main.go` — no changes (same scraper dispatch map, same CLI flags)
- All XMLTV output format — identical
- T2 and A1 scrapers — no changes
