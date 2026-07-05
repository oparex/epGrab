# tvsporedi FlareSolverr Scraper Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Re-add tvsporedi.si EPG scraping using FlareSolverr as the Cloudflare bypass, fetching today's schedule only.

**Architecture:** `flaresolverr.go` is a self-contained FlareSolverr HTTP client with session management (create / request.get / destroy). `tvsporedi.go` uses that client and goquery to parse channels and today's schedule, following the same `runXXX(outPath string)` convention as `siol.go`. `main.go` gains `-flaresolverr-url` and `-dry-run` flags; the scrapers map is moved inside `main()` so the tvsporedi closure can capture the flag values.

**Tech Stack:** Go 1.26, `net/http`, `encoding/json`, `github.com/PuerkitoBio/goquery`, `net/http/httptest` (tests only — no new runtime deps)

## Global Constraints

- Package: `main` (all files in repo root)
- No new external dependencies — only stdlib + goquery (already in `go.mod`)
- No colly, no chromedp
- Output: `epg_tvsporedi.xml` (XMLTV format, same as `siol`/`t2`/`a1`)
- Schedule depth: today only (1 day)
- All functions package-private (lowercase first letter)
- Error handling in `runTvSporedi`: `fmt.Printf` + `return` (not `log.Fatalf`) so `-scraper all` continues with other scrapers when FlareSolverr is unreachable

---

### Task 1: FlareSolverr HTTP client

**Files:**
- Create: `flaresolverr.go`
- Create: `flaresolverr_test.go`

**Interfaces:**
- Produces:
  - `fsRequest` struct (json tags: `cmd`, `url`, `maxTimeout`, `session` — all `omitempty` except `cmd`)
  - `fsSolution` struct (json tags: `status int`, `response string`)
  - `fsResponse` struct (json tags: `status string`, `message string`, `session string`, `solution fsSolution`)
  - `flaresolverrGetHTML(targetURL, endpoint, sessionID string) (string, error)`
  - `flaresolverrCreateSession(endpoint string) (string, error)`
  - `flaresolverrDestroySession(endpoint, sessionID string) error`

---

- [ ] **Step 1: Write failing tests**

Create `flaresolverr_test.go`:

```go
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlaresolverrGetHTML_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1", r.URL.Path)

		var req fsRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "request.get", req.Cmd)
		assert.Equal(t, "http://example.com", req.URL)
		assert.Equal(t, "sess1", req.Session)

		json.NewEncoder(w).Encode(fsResponse{
			Status:   "ok",
			Solution: fsSolution{Status: 200, Response: "<html>hello</html>"},
		})
	}))
	defer srv.Close()

	html, err := flaresolverrGetHTML("http://example.com", srv.URL, "sess1")
	require.NoError(t, err)
	assert.Equal(t, "<html>hello</html>", html)
}

func TestFlaresolverrGetHTML_noSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req fsRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "", req.Session, "empty sessionID must not send session field")
		json.NewEncoder(w).Encode(fsResponse{
			Status:   "ok",
			Solution: fsSolution{Status: 200, Response: "<html>ok</html>"},
		})
	}))
	defer srv.Close()

	_, err := flaresolverrGetHTML("http://example.com", srv.URL, "")
	require.NoError(t, err)
}

func TestFlaresolverrGetHTML_fsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(fsResponse{Status: "error", Message: "session not found"})
	}))
	defer srv.Close()

	_, err := flaresolverrGetHTML("http://example.com", srv.URL, "bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestFlaresolverrGetHTML_targetNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(fsResponse{
			Status:   "ok",
			Solution: fsSolution{Status: 403, Response: ""},
		})
	}))
	defer srv.Close()

	_, err := flaresolverrGetHTML("http://example.com", srv.URL, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

func TestFlaresolverrCreateSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req fsRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "sessions.create", req.Cmd)
		json.NewEncoder(w).Encode(fsResponse{Status: "ok", Session: "abc123"})
	}))
	defer srv.Close()

	sessionID, err := flaresolverrCreateSession(srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "abc123", sessionID)
}

func TestFlaresolverrDestroySession(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req fsRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "sessions.destroy", req.Cmd)
		assert.Equal(t, "abc123", req.Session)
		called = true
		json.NewEncoder(w).Encode(fsResponse{Status: "ok"})
	}))
	defer srv.Close()

	require.NoError(t, flaresolverrDestroySession(srv.URL, "abc123"))
	assert.True(t, called)
}
```

- [ ] **Step 2: Run tests — confirm they fail to compile**

```bash
go test ./... -run TestFlaresolverr
```

Expected: compilation error `undefined: fsRequest` (or similar). That's the TDD red state.

- [ ] **Step 3: Implement `flaresolverr.go`**

Create `flaresolverr.go`:

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var fsHTTPClient = &http.Client{
	Timeout: 90 * time.Second,
}

type fsRequest struct {
	Cmd        string `json:"cmd"`
	URL        string `json:"url,omitempty"`
	MaxTimeout int    `json:"maxTimeout,omitempty"`
	Session    string `json:"session,omitempty"`
}

type fsSolution struct {
	Status   int    `json:"status"`
	Response string `json:"response"`
}

type fsResponse struct {
	Status   string     `json:"status"`
	Message  string     `json:"message"`
	Session  string     `json:"session,omitempty"`
	Solution fsSolution `json:"solution"`
}

func flaresolverrPost(endpoint string, req fsRequest) (fsResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return fsResponse{}, err
	}
	var lastErr error
	for attempt := 0; attempt < httpMaxAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(httpBaseBackoff << (attempt - 1))
		}
		resp, retryable, err := flaresolverrPostOnce(endpoint, body)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !retryable {
			return fsResponse{}, err
		}
	}
	return fsResponse{}, fmt.Errorf("after %d attempts: %w", httpMaxAttempts, lastErr)
}

func flaresolverrPostOnce(endpoint string, body []byte) (fsResponse, bool, error) {
	httpResp, err := fsHTTPClient.Post(endpoint+"/v1", "application/json", bytes.NewReader(body))
	if err != nil {
		return fsResponse{}, true, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		retryable := httpResp.StatusCode >= 500
		return fsResponse{}, retryable, fmt.Errorf("flaresolverr HTTP %d from %s/v1", httpResp.StatusCode, endpoint)
	}

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fsResponse{}, true, err
	}

	var res fsResponse
	if err := json.Unmarshal(raw, &res); err != nil {
		return fsResponse{}, false, fmt.Errorf("flaresolverr response parse: %w", err)
	}
	return res, false, nil
}

func flaresolverrGetHTML(targetURL, endpoint, sessionID string) (string, error) {
	resp, err := flaresolverrPost(endpoint, fsRequest{
		Cmd:        "request.get",
		URL:        targetURL,
		MaxTimeout: 60000,
		Session:    sessionID,
	})
	if err != nil {
		return "", fmt.Errorf("flaresolverr fetch %s: %w", targetURL, err)
	}
	if resp.Status != "ok" {
		return "", fmt.Errorf("flaresolverr error for %s: %s", targetURL, resp.Message)
	}
	if resp.Solution.Status != http.StatusOK {
		return "", fmt.Errorf("target returned HTTP %d for %s", resp.Solution.Status, targetURL)
	}
	return resp.Solution.Response, nil
}

func flaresolverrCreateSession(endpoint string) (string, error) {
	resp, err := flaresolverrPost(endpoint, fsRequest{Cmd: "sessions.create"})
	if err != nil {
		return "", fmt.Errorf("flaresolverr create session: %w", err)
	}
	if resp.Status != "ok" {
		return "", fmt.Errorf("flaresolverr create session failed: %s", resp.Message)
	}
	return resp.Session, nil
}

func flaresolverrDestroySession(endpoint, sessionID string) error {
	_, err := flaresolverrPost(endpoint, fsRequest{Cmd: "sessions.destroy", Session: sessionID})
	return err
}
```

- [ ] **Step 4: Run tests — confirm they pass**

```bash
go test ./... -run TestFlaresolverr -v
```

Expected: all 6 `TestFlaresolverr*` tests PASS.

- [ ] **Step 5: Commit**

```bash
git add flaresolverr.go flaresolverr_test.go
git commit -m "feat: add FlareSolverr HTTP client with session management"
```

---

### Task 2: tvsporedi HTML parser functions + test fixtures

**Files:**
- Create: `testdata/tvsporedi_channels.html`
- Create: `testdata/tvsporedi_schedule.html`
- Create: `tvsporedi.go` (parser functions only — `runTvSporedi` added in Task 3)
- Create: `tvsporedi_test.go` (parser tests — integration test added in Task 3)

**Interfaces:**
- Consumes: `parseTime(y int, m time.Month, d int, hour string) string` from `fetch.go`
- Produces:
  - `parseTvSporediChannels(doc *goquery.Document) []string`
  - `parseTvSporediSchedule(doc *goquery.Document, chanID string, y int, m time.Month, d int) []*Programme`
  - `isValidTvSporediTime(s string) bool`

---

- [ ] **Step 1: Create fixture HTML files**

Create `testdata/tvsporedi_channels.html`:

```html
<!DOCTYPE html>
<html>
<body>
<div id="left-navigation">
  <a href="?id=SLO1&amp;dan=a">SLO 1</a>
  <a href="?id=SLO2&amp;dan=a">SLO 2</a>
  <a href="?id=POP_TV&amp;dan=a">POP TV</a>
  <a href="?id=SLO1&amp;dan=b">SLO 1 (dupe)</a>
  <a href="/other-link">Not a channel</a>
</div>
</body>
</html>
```

Create `testdata/tvsporedi_schedule.html`:

```html
<!DOCTYPE html>
<html>
<body>
<div id="a">
  <div class="schedule">
    <div>
      <span class="time">06:00</span>
      <span class="prog">Jutarnji program</span>
    </div>
    <div class="clear"></div>
    <div>
      <span class="time">08:30</span>
      <span class="prog">Oddaja za otroke</span>
    </div>
    <div>
      <span class="time">bad-time</span>
      <span class="prog">Should be skipped</span>
    </div>
  </div>
</div>
<div id="b">
  <div class="schedule">
    <div>
      <span class="time">06:00</span>
      <span class="prog">Tomorrow show — must not appear</span>
    </div>
  </div>
</div>
</body>
</html>
```

- [ ] **Step 2: Write failing parser tests**

Create `tvsporedi_test.go`:

```go
package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadTvSporediDoc(t *testing.T, path string) *goquery.Document {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(b)))
	require.NoError(t, err)
	return doc
}

func TestParseTvSporediChannels(t *testing.T) {
	doc := loadTvSporediDoc(t, "testdata/tvsporedi_channels.html")
	channels := parseTvSporediChannels(doc)
	assert.Equal(t, []string{"SLO1", "SLO2", "POP_TV"}, channels)
}

func TestParseTvSporediChannels_dedup(t *testing.T) {
	doc := loadTvSporediDoc(t, "testdata/tvsporedi_channels.html")
	channels := parseTvSporediChannels(doc)
	seen := make(map[string]int)
	for _, ch := range channels {
		seen[ch]++
	}
	for id, count := range seen {
		assert.Equal(t, 1, count, "channel %s appears more than once", id)
	}
}

func TestParseTvSporediSchedule(t *testing.T) {
	doc := loadTvSporediDoc(t, "testdata/tvsporedi_schedule.html")
	programmes := parseTvSporediSchedule(doc, "SLO1", 2026, time.July, 5)

	require.Len(t, programmes, 2, "bad-time row and tomorrow row must be excluded")

	assert.Equal(t, "SLO1", programmes[0].Channel)
	assert.Equal(t, "20260705060000 +0200", programmes[0].Start)
	assert.Equal(t, "Jutarnji program", programmes[0].Title.Value)
	assert.Equal(t, "sl", programmes[0].Title.Lang)

	assert.Equal(t, "20260705083000 +0200", programmes[1].Start)
	assert.Equal(t, "Oddaja za otroke", programmes[1].Title.Value)
}

func TestIsValidTvSporediTime(t *testing.T) {
	assert.True(t, isValidTvSporediTime("06:00"))
	assert.True(t, isValidTvSporediTime("23:59"))
	assert.False(t, isValidTvSporediTime("bad-time"))
	assert.False(t, isValidTvSporediTime("6:00"))
	assert.False(t, isValidTvSporediTime(""))
	assert.False(t, isValidTvSporediTime("06:0"))
}
```

- [ ] **Step 3: Run tests — confirm they fail to compile**

```bash
go test ./... -run "TestParseTvSporedi|TestIsValid"
```

Expected: compilation error `undefined: parseTvSporediChannels`. That is the TDD red state.

- [ ] **Step 4: Implement parser functions in `tvsporedi.go`**

Create `tvsporedi.go`:

```go
package main

import (
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// parseTvSporediChannels extracts unique channel IDs from the tvsporedi main
// page. Each channel link is an <a href="?id=CHANNEL&dan=a"> inside
// #left-navigation; the id= query param is the channel identifier.
func parseTvSporediChannels(doc *goquery.Document) []string {
	var channels []string
	seen := make(map[string]bool)

	doc.Find("#left-navigation a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		u, err := url.Parse(href)
		if err != nil {
			return
		}
		chanID := u.Query().Get("id")
		if chanID == "" || seen[chanID] {
			return
		}
		seen[chanID] = true
		channels = append(channels, chanID)
	})
	return channels
}

// parseTvSporediSchedule extracts today's programmes from a channel schedule
// page. The page wraps today's block in <div id="a">; tomorrow is "b", the
// day after "c". Since we only fetch today we only parse #a.
func parseTvSporediSchedule(doc *goquery.Document, chanID string, y int, m time.Month, d int) []*Programme {
	var out []*Programme

	doc.Find("#a .schedule > div").Each(func(_ int, s *goquery.Selection) {
		if s.HasClass("clear") {
			return
		}
		timeStr := strings.TrimSpace(s.Find(".time").First().Text())
		title := strings.TrimSpace(s.Find(".prog").First().Text())
		if !isValidTvSporediTime(timeStr) || title == "" {
			return
		}
		out = append(out, &Programme{
			Channel: chanID,
			Start:   parseTime(y, m, d, timeStr),
			Title: &Title{
				Lang:  "sl",
				Value: title,
			},
		})
	})
	return out
}

// isValidTvSporediTime reports whether s is a well-formed HH:MM clock string.
func isValidTvSporediTime(s string) bool {
	parts := strings.Split(s, ":")
	if len(parts) != 2 || len(parts[0]) != 2 || len(parts[1]) != 2 {
		return false
	}
	for _, r := range parts[0] + parts[1] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
```

- [ ] **Step 5: Run tests — confirm they pass**

```bash
go test ./... -run "TestParseTvSporedi|TestIsValid" -v
```

Expected: all 4 tests PASS.

- [ ] **Step 6: Commit**

```bash
git add testdata/tvsporedi_channels.html testdata/tvsporedi_schedule.html \
        tvsporedi.go tvsporedi_test.go
git commit -m "feat: add tvsporedi HTML parser functions + test fixtures"
```

---

### Task 3: `runTvSporedi` orchestration + `main.go` wiring

**Files:**
- Modify: `tvsporedi.go` — add `runTvSporedi` (new imports: `encoding/xml`, `fmt`, `net/url`, `os`)
- Modify: `tvsporedi_test.go` — add integration test (new imports: `encoding/json`, `io`, `net/http`, `net/http/httptest`)
- Modify: `main.go` — move scrapers map inside `main()`, add flags, register tvsporedi

**Interfaces:**
- Consumes: `flaresolverrCreateSession`, `flaresolverrGetHTML`, `flaresolverrDestroySession` from `flaresolverr.go`
- Consumes: `parseTvSporediChannels`, `parseTvSporediSchedule` from `tvsporedi.go` (Task 2)
- Consumes: `Tv`, `Channel`, `DisplayName`, `Programme` from `schema.go`

---

- [ ] **Step 1: Write failing integration test**

Add to `tvsporedi_test.go` (append after existing tests, adding required imports at top):

Update the import block in `tvsporedi_test.go` to:

```go
import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
```

Append these two tests to `tvsporedi_test.go`:

```go
func mockFlaresolverrServer(t *testing.T, channelsHTML, scheduleHTML string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req fsRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		switch req.Cmd {
		case "sessions.create":
			json.NewEncoder(w).Encode(fsResponse{Status: "ok", Session: "test-sess"})
		case "sessions.destroy":
			json.NewEncoder(w).Encode(fsResponse{Status: "ok"})
		case "request.get":
			body := scheduleHTML
			if req.URL == "https://www.tvsporedi.si/" {
				body = channelsHTML
			}
			json.NewEncoder(w).Encode(fsResponse{
				Status:   "ok",
				Solution: fsSolution{Status: 200, Response: body},
			})
		}
	}))
}

func TestRunTvSporedi_writesXML(t *testing.T) {
	channelsHTML, err := os.ReadFile("testdata/tvsporedi_channels.html")
	require.NoError(t, err)
	scheduleHTML, err := os.ReadFile("testdata/tvsporedi_schedule.html")
	require.NoError(t, err)

	srv := mockFlaresolverrServer(t, string(channelsHTML), string(scheduleHTML))
	defer srv.Close()

	dir := t.TempDir()
	runTvSporedi(dir+"/", srv.URL, false)

	xmlData, err := os.ReadFile(dir + "/epg_tvsporedi.xml")
	require.NoError(t, err)
	xml := string(xmlData)
	assert.Contains(t, xml, `<channel id="SLO1">`)
	assert.Contains(t, xml, `<channel id="SLO2">`)
	assert.Contains(t, xml, `<channel id="POP_TV">`)
	assert.Contains(t, xml, "Jutarnji program")
	assert.Contains(t, xml, "Oddaja za otroke")
}

func TestRunTvSporedi_dryRun_noFile(t *testing.T) {
	channelsHTML, err := os.ReadFile("testdata/tvsporedi_channels.html")
	require.NoError(t, err)
	scheduleHTML, err := os.ReadFile("testdata/tvsporedi_schedule.html")
	require.NoError(t, err)

	srv := mockFlaresolverrServer(t, string(channelsHTML), string(scheduleHTML))
	defer srv.Close()

	dir := t.TempDir()
	runTvSporedi(dir+"/", srv.URL, true)

	_, err = os.Stat(dir + "/epg_tvsporedi.xml")
	assert.True(t, os.IsNotExist(err), "dry run must not write epg_tvsporedi.xml")
}
```

- [ ] **Step 2: Run tests — confirm they fail to compile**

```bash
go test ./... -run TestRunTvSporedi
```

Expected: compilation error `undefined: runTvSporedi`. That is the TDD red state.

- [ ] **Step 3: Implement `runTvSporedi` — update `tvsporedi.go`**

Replace the `tvsporedi.go` file with the complete version (all parser functions from Task 2 plus `runTvSporedi`):

```go
package main

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// runTvSporedi is the entry point for the tvsporedi scraper. It creates a
// FlareSolverr session (solving the Cloudflare managed challenge once), fetches
// today's schedule for every channel, and writes epg_tvsporedi.xml. On any
// fatal error it prints and returns rather than calling log.Fatalf so that
// -scraper all can continue with the remaining scrapers.
func runTvSporedi(outPath, endpoint string, dryRun bool) {
	sessionID, err := flaresolverrCreateSession(endpoint)
	if err != nil {
		fmt.Printf("tvsporedi: create FlareSolverr session at %s: %s\n", endpoint, err)
		return
	}
	defer func() {
		if err := flaresolverrDestroySession(endpoint, sessionID); err != nil {
			fmt.Printf("tvsporedi: destroy session warning: %s\n", err)
		}
	}()

	mainHTML, err := flaresolverrGetHTML("https://www.tvsporedi.si/", endpoint, sessionID)
	if err != nil {
		fmt.Printf("tvsporedi: fetch main page: %s\n", err)
		return
	}

	mainDoc, err := goquery.NewDocumentFromReader(strings.NewReader(mainHTML))
	if err != nil {
		fmt.Printf("tvsporedi: parse main page: %s\n", err)
		return
	}

	channelIDs := parseTvSporediChannels(mainDoc)
	if len(channelIDs) == 0 {
		fmt.Println("tvsporedi: no channels found — selectors may need updating for current site HTML")
		return
	}

	tv := &Tv{
		GeneratorInfoName: "peter",
		GeneratorInfoUrl:  "peter",
	}

	y, m, d := time.Now().Date()

	for _, chanID := range channelIDs {
		fmt.Println(chanID)

		tv.Channel = append(tv.Channel, &Channel{
			Id: chanID,
			DisplayName: &DisplayName{
				Lang:  "sl",
				Value: chanID,
			},
		})

		schedURL := fmt.Sprintf("https://www.tvsporedi.si/?id=%s", url.QueryEscape(chanID))
		schedHTML, err := flaresolverrGetHTML(schedURL, endpoint, sessionID)
		if err != nil {
			fmt.Printf("tvsporedi: fetch schedule for %s: %s\n", chanID, err)
			continue
		}

		schedDoc, err := goquery.NewDocumentFromReader(strings.NewReader(schedHTML))
		if err != nil {
			fmt.Printf("tvsporedi: parse schedule for %s: %s\n", chanID, err)
			continue
		}

		programmes := parseTvSporediSchedule(schedDoc, chanID, y, m, d)

		var prev *Programme
		for _, p := range programmes {
			if prev != nil {
				prev.Stop = p.Start
			}
			prev = p
			tv.Programme = append(tv.Programme, p)
		}
	}

	if dryRun {
		fmt.Printf("tvsporedi: dry run — %d channels, %d programmes (no XML written)\n",
			len(tv.Channel), len(tv.Programme))
		return
	}

	out, err := xml.MarshalIndent(tv, " ", "  ")
	if err != nil {
		fmt.Printf("tvsporedi: marshal XML: %s\n", err)
		return
	}

	if err := os.WriteFile(outPath+"epg_tvsporedi.xml", out, 0644); err != nil {
		fmt.Printf("tvsporedi: write XML: %s\n", err)
		return
	}

	fmt.Printf("tvsporedi: wrote %s\n", outPath+"epg_tvsporedi.xml")
}

// parseTvSporediChannels extracts unique channel IDs from the tvsporedi main
// page. Each channel link is an <a href="?id=CHANNEL&dan=a"> inside
// #left-navigation; the id= query param is the channel identifier.
func parseTvSporediChannels(doc *goquery.Document) []string {
	var channels []string
	seen := make(map[string]bool)

	doc.Find("#left-navigation a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		u, err := url.Parse(href)
		if err != nil {
			return
		}
		chanID := u.Query().Get("id")
		if chanID == "" || seen[chanID] {
			return
		}
		seen[chanID] = true
		channels = append(channels, chanID)
	})
	return channels
}

// parseTvSporediSchedule extracts today's programmes from a channel schedule
// page. The page wraps today's block in <div id="a">; tomorrow is "b", the
// day after "c". Since we only fetch today we only parse #a.
func parseTvSporediSchedule(doc *goquery.Document, chanID string, y int, m time.Month, d int) []*Programme {
	var out []*Programme

	doc.Find("#a .schedule > div").Each(func(_ int, s *goquery.Selection) {
		if s.HasClass("clear") {
			return
		}
		timeStr := strings.TrimSpace(s.Find(".time").First().Text())
		title := strings.TrimSpace(s.Find(".prog").First().Text())
		if !isValidTvSporediTime(timeStr) || title == "" {
			return
		}
		out = append(out, &Programme{
			Channel: chanID,
			Start:   parseTime(y, m, d, timeStr),
			Title: &Title{
				Lang:  "sl",
				Value: title,
			},
		})
	})
	return out
}

// isValidTvSporediTime reports whether s is a well-formed HH:MM clock string.
func isValidTvSporediTime(s string) bool {
	parts := strings.Split(s, ":")
	if len(parts) != 2 || len(parts[0]) != 2 || len(parts[1]) != 2 {
		return false
	}
	for _, r := range parts[0] + parts[1] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: Update `main.go`**

Replace `main.go` entirely (moves the scrapers map inside `main()` so the closure can capture flag values):

```go
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	outPath := flag.String("out", "./", "Path to output xml")
	scraper := flag.String("scraper", "", "Scraper to run: tvsporedi, t2, siol, a1, or 'all' to run all")
	flaresolverrURL := flag.String("flaresolverr-url", "http://localhost:8191", "FlareSolverr base URL (required for tvsporedi)")
	dryRun := flag.Bool("dry-run", false, "Print parsed data without writing XML (tvsporedi only)")
	flag.Parse()

	scrapers := map[string]func(string){
		"t2":   runT2,
		"siol": runSiol,
		"a1":   runA1,
		"tvsporedi": func(outPath string) {
			runTvSporedi(outPath, *flaresolverrURL, *dryRun)
		},
	}

	if *scraper == "" {
		fmt.Println("Error: -scraper flag is required")
		fmt.Println("\nAvailable scrapers:")
		fmt.Println("  tvsporedi  - TvSporedi.si HTML scraper (requires -flaresolverr-url)")
		fmt.Println("  t2         - T-2 JSON API scraper")
		fmt.Println("  siol       - Siol HTML scraper")
		fmt.Println("  a1         - A1 Slovenija JSON API scraper")
		fmt.Println("  all        - Run all scrapers")
		fmt.Println("\nUsage: epGrab -scraper <name> [-out <path>] [-flaresolverr-url <url>] [-dry-run]")
		os.Exit(1)
	}

	if *scraper == "all" {
		for name, runFunc := range scrapers {
			fmt.Printf("Running %s scraper...\n", name)
			runFunc(*outPath)
		}
		return
	}

	scraperName := strings.ToLower(*scraper)
	runFunc, exists := scrapers[scraperName]
	if !exists {
		fmt.Printf("Error: unknown scraper '%s'\n", *scraper)
		fmt.Println("\nAvailable scrapers: tvsporedi, t2, siol, a1, all")
		os.Exit(1)
	}

	fmt.Printf("Running %s scraper...\n", scraperName)
	runFunc(*outPath)
}
```

- [ ] **Step 5: Run all tests — confirm they pass**

```bash
go test ./... -v
```

Expected: all tests PASS (`TestFlaresolverr*`, `TestParseTvSporedi*`, `TestIsValid*`, `TestRunTvSporedi*`, plus existing siol/t2 tests).

- [ ] **Step 6: Build the binary to confirm it compiles**

```bash
go build -o epGrab_test_build . && rm epGrab_test_build
```

Expected: no output (clean build).

- [ ] **Step 7: Verify help text includes tvsporedi and new flags**

```bash
go run . -scraper ""
```

Expected output includes:
```
  tvsporedi  - TvSporedi.si HTML scraper (requires -flaresolverr-url)
```
and the `-flaresolverr-url` and `-dry-run` flags appear in `--help`.

- [ ] **Step 8: Commit**

```bash
git add tvsporedi.go tvsporedi_test.go main.go
git commit -m "feat: add tvsporedi scraper via FlareSolverr with dry-run support"
```
