# tvsporedi scraper via FlareSolverr

**Date:** 2026-07-05  
**Status:** Approved

## Problem

`www.tvsporedi.si` is behind a Cloudflare managed challenge (`cf-mitigated: challenge`, HTTP 403). Plain Go HTTP and TLS-fingerprint impersonation cannot pass it — real JS/Turnstile execution is required. Chrome is not available on the deploy host, so chromedp is off the table.

## Solution

Route all tvsporedi fetches through a FlareSolverr instance running elsewhere (Docker container, separate VPS, dev machine). FlareSolverr exposes a simple HTTP API: the Go binary POSTs a URL, FlareSolverr solves the CF challenge internally, and returns the rendered HTML. No Chrome on the main host.

## Architecture

Two new files, two files modified, one unchanged:

| File | Role |
|---|---|
| `flaresolverr.go` | **New.** FlareSolverr HTTP client |
| `tvsporedi.go` | **New.** Scraper using goquery |
| `main.go` | Add `-flaresolverr-url` flag; register tvsporedi scraper |
| `fetch.go` | **Unchanged** |

## flaresolverr.go

Core function: `flaresolverrGetHTML(targetURL, endpoint, sessionID string) (string, error)` — `sessionID` may be empty string for sessionless requests.

**Protocol:**
- `POST {endpoint}/v1`
- Request body: `{"cmd": "request.get", "url": "...", "maxTimeout": 60000}` (plus `"session": "<id>"` when sessionID is non-empty)
- Response: JSON with `status` and `solution.response` (HTML string) and `solution.status` (HTTP status code)

**HTTP client:** Dedicated `http.Client` with 90s timeout (FlareSolverr can take 5–20s to solve a challenge; the shared 30s `httpClient` would cut it off).

**Retries:** Reuse `httpMaxAttempts` and `httpBaseBackoff` from `fetch.go` for transport-level and 5xx errors.

**Session management:** FlareSolverr v2 sessions allow one challenge-solve to be reused across many requests (critical for performance — without sessions, ~50 channel fetches × 5–20s solve time = up to 1000s total):

- `sessions.create` → returns session ID
- All `request.get` calls include `"session": id`
- `sessions.destroy` called via `defer` to clean up even on error

Session helpers: `flaresolverrCreateSession(endpoint string) (string, error)` and `flaresolverrDestroySession(endpoint, sessionID string) error`

## tvsporedi.go

**Entry point:** `runTvSporedi(outPath, endpoint string, dryRun bool)`

**Flow:**
1. Create FlareSolverr session
2. `defer` session destroy
3. Fetch `https://www.tvsporedi.si/` — parse channel list from `#left-navigation a[href*="id="]`, extract `id=` query param as channel ID
4. For each channel, fetch `https://www.tvsporedi.si/?id={channelID}` (today's schedule)
5. Parse schedule with goquery: `.schedule > div` (skip `.clear` divs), extract time from `.time` child and title from `.prog` child
6. Build `Programme` structs using `parseTime` from `fetch.go`
7. If `dryRun`: print channels and programme counts, return without writing XML
8. Write `epg_tvsporedi.xml`

**HTML parsing:** goquery only — no colly. Selectors based on the 2024 colly scraper's patterns; may need adjustment after first live test run against the current site structure.

**Output:** `epg_tvsporedi.xml` — same XMLTV format as other scrapers.

**Schedule depth:** Today only (1 day).

## main.go changes

**New flag:**
```go
flaresolverrURL := flag.String("flaresolverr-url", "http://localhost:8191", "FlareSolverr base URL")
dryRun := flag.Bool("dry-run", false, "Print parsed data without writing XML")
```

**Registration via closure** (keeps `map[string]func(string)` type unchanged):
```go
scrapers["tvsporedi"] = func(outPath string) { runTvSporedi(outPath, *flaresolverrURL, *dryRun) }
```

**"all" mode:** tvsporedi runs alongside other scrapers. If FlareSolverr is unreachable, tvsporedi logs the error and the "all" run continues with the remaining scrapers (same behaviour as a per-channel fetch failure — one scraper's failure does not abort the rest).

## Error handling

- FlareSolverr unreachable: fail fast with a descriptive error (includes endpoint URL)
- `solution.status != 200`: treat as a fetch error, retry if 5xx
- `status != "ok"` from FlareSolverr: non-retryable error (bad request, session expired)
- Session create failure: abort `runTvSporedi` entirely (no point fetching without a session)
- Per-channel fetch failure: log and skip that channel, continue with others

## Testing / verification

Since the site is CF-protected, selectors cannot be verified without FlareSolverr running. Use `--dry-run` after standing up FlareSolverr to confirm channel list and programme parsing before committing to XML output:

```sh
docker run -d --name flaresolverr -p 8191:8191 ghcr.io/flaresolverr/flaresolverr:latest
./epGrab -scraper tvsporedi -flaresolverr-url http://localhost:8191 -dry-run
```

If selectors need adjustment, the HTML is visible in FlareSolverr logs (`docker logs flaresolverr`).
