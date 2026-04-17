# chromedp Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Colly with chromedp + goquery in the HTML scrapers (tvsporedi, siol) so they can bypass Cloudflare protection, and fully implement the siol scraper.

**Architecture:** Each HTML scraper uses a two-phase approach: chromedp launches a headless browser to fetch the fully-rendered page HTML (handling JS challenges), then goquery parses the HTML using CSS selectors. A shared `browser.go` provides the `fetchRenderedHTML` helper.

**Tech Stack:** Go, chromedp, goquery, golang.org/x/net/html

---

### Task 1: Create branch and update Go module dependencies

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Create feature branch**

```bash
git checkout -b feature/chromedp-refactor
```

- [ ] **Step 2: Bump Go version and add chromedp dependency**

Update `go.mod` to require Go 1.21 and add chromedp + promote goquery to direct dep:

```bash
go mod edit -go=1.21
go get github.com/chromedp/chromedp@latest
go get github.com/PuerkitoBio/goquery@latest
```

- [ ] **Step 3: Remove Colly dependency**

```bash
go get github.com/gocolly/colly@none
```

- [ ] **Step 4: Tidy modules**

```bash
go mod tidy
```

- [ ] **Step 5: Verify the module file looks correct**

Run: `cat go.mod`
Expected: `go 1.21` (or higher), `chromedp` and `goquery` in require block, no `colly`.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: switch deps from colly to chromedp + goquery"
```

---

### Task 2: Create browser.go shared helper

**Files:**
- Create: `browser.go`

- [ ] **Step 1: Create browser.go with fetchRenderedHTML**

Create `browser.go`:

```go
package main

import (
	"context"
	"time"

	"github.com/chromedp/chromedp"
)

func fetchRenderedHTML(targetURL string) (string, error) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var htmlContent string
	err := chromedp.Run(ctx,
		chromedp.Navigate(targetURL),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.OuterHTML("html", &htmlContent, chromedp.ByQuery),
	)
	if err != nil {
		return "", err
	}

	return htmlContent, nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: No errors (tvsporedi.go and siol.go will fail because they still import colly — that's expected at this stage, we handle them in the next tasks).

Note: This step will fail until Tasks 3 and 4 remove the colly imports. That's fine — just verify `browser.go` itself has no syntax errors by checking the compiler output mentions only colly-related errors in other files.

- [ ] **Step 3: Commit**

```bash
git add browser.go
git commit -m "feat: add browser.go with chromedp fetchRenderedHTML helper"
```

---

### Task 3: Refactor tvsporedi.go from Colly to chromedp + goquery

**Files:**
- Modify: `tvsporedi.go`

- [ ] **Step 1: Rewrite tvsporedi.go**

Replace the entire content of `tvsporedi.go` with:

```go
package main

import (
	"encoding/xml"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

func parseTime(year int, month time.Month, day int, hour string) string {
	return fmt.Sprintf("%d%02d%02d%s00 +0200", year, month, day, strings.ReplaceAll(hour, ":", ""))
}

func findFullPrevSibling(currentNode *html.Node) *html.Node {
	if currentNode.PrevSibling == nil || currentNode.PrevSibling.Data == "\n\t\t\t\t\t\t" || currentNode.PrevSibling.Data == "a" {
		return nil
	}
	if len(currentNode.PrevSibling.Attr) > 0 && currentNode.PrevSibling.Attr[0].Key == "class" && currentNode.PrevSibling.Attr[0].Val == "clear" {
		return findFullPrevSibling(currentNode.PrevSibling)
	}
	return currentNode.PrevSibling
}

func timeOffset(currentNode *html.Node) int {
	lastNode := findFullPrevSibling(currentNode)
	if lastNode == nil {
		return 0
	}
	if lastNode.FirstChild.FirstChild.Data > currentNode.FirstChild.FirstChild.Data {
		return 1
	}
	return timeOffset(lastNode)
}

func runTvSporedi(outPath string) {
	tv := &Tv{
		GeneratorInfoName: "peter",
		GeneratorInfoUrl:  "peter",
	}

	// Fetch main page
	mainHTML, err := fetchRenderedHTML("https://www.tvsporedi.si/spored.php")
	if err != nil {
		log.Fatalf("Failed to fetch tvsporedi main page: %s", err)
	}

	mainDoc, err := goquery.NewDocumentFromReader(strings.NewReader(mainHTML))
	if err != nil {
		log.Fatalf("Failed to parse tvsporedi main page: %s", err)
	}

	// Extract channel links
	type channelInfo struct {
		id  string
		url string
	}
	var channels []channelInfo

	mainDoc.Find("div[id=left-navigation] a[href]").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}
		params, err := url.ParseQuery(href)
		if err != nil {
			log.Fatalf("Problem decoding url %s: %s", href, err)
		}
		if len(params["id"]) == 0 {
			log.Fatalf("Empty id param in url %s", href)
		}
		chanID := params["id"][0]
		fmt.Println(chanID)

		tv.Channel = append(tv.Channel, &Channel{
			Id: chanID,
			DisplayName: &DisplayName{
				Lang:  "sl",
				Value: chanID,
			},
		})

		channelURL := "https://www.tvsporedi.si/" + strings.ReplaceAll(href, " ", "+")
		channels = append(channels, channelInfo{id: chanID, url: channelURL})
	})

	// Fetch each channel's schedule
	for _, ch := range channels {
		chHTML, err := fetchRenderedHTML(ch.url)
		if err != nil {
			fmt.Printf("Failed to fetch channel %s: %s\n", ch.id, err)
			continue
		}

		chDoc, err := goquery.NewDocumentFromReader(strings.NewReader(chHTML))
		if err != nil {
			fmt.Printf("Failed to parse channel %s: %s\n", ch.id, err)
			continue
		}

		chDoc.Find("div.schedule > div:not(.clear)").Each(func(_ int, s *goquery.Selection) {
			node := s.Nodes[0]

			hour := s.Find(".time").Text()
			prog := s.Find(".prog").Text()
			rawData := getDesc(node.LastChild)

			if strings.Count(hour, ":") > 1 {
				return
			}

			year, month, day := time.Now().Date()
			parent := node.Parent.Parent
			if len(parent.Attr) > 0 && parent.Attr[0].Key == "id" {
				if parent.Attr[0].Val == "b" {
					day += 1
				}
				if parent.Attr[0].Val == "c" {
					day += 2
				}
			} else {
				return
			}
			day += timeOffset(node)

			p := &Programme{
				Channel: ch.id,
				Start:   parseTime(year, month, day, hour),
				Description: &Description{
					Lang:  "sl",
					Value: rawData,
				},
				Title: &Title{
					Lang:  "sl",
					Value: prog,
				},
			}
			if len(tv.Programme) > 0 {
				tv.Programme[len(tv.Programme)-1].Stop = p.Start
			}
			tv.Programme = append(tv.Programme, p)
		})
	}

	out, err := xml.MarshalIndent(tv, " ", "  ")
	if err != nil {
		fmt.Println(err)
	}

	err = os.WriteFile(outPath+"epg_tvsporedi.xml", out, 0644)
	if err != nil {
		log.Fatalf("Error writing to file: %s", err)
	}
}

func getDesc(startNode *html.Node) string {
	currentNode := startNode.FirstChild

	var result string
	for {
		if currentNode == nil {
			break
		}
		if currentNode.Data == "br" {
			if len(result) > 0 && result[len(result)-1] != '.' {
				result += ". "
			}
		} else if currentNode.Data == "p" && currentNode.FirstChild != nil {
			result += currentNode.FirstChild.Data
		} else {
			result += currentNode.Data
		}
		currentNode = currentNode.NextSibling
	}
	return result
}

func processData(p *Programme, result []string) {
	for _, s := range result {
		s = strings.ReplaceAll(s, "&#39;", "'")
		if isEpisodeInfo(s) {
			p.EpisodeNumber = &EpisodeNumber{
				Value: s,
			}
		} else if isCountryInfo(s) {
			p.Country = &Country{
				Lang:  "sl",
				Value: s,
			}
		} else if isCastInfo(s) {
			credits := &Credits{}
			for _, actor := range strings.Split(s[3:], ", ") {
				credits.Actor = append(credits.Actor, &Actor{Value: actor})
			}
			p.Credits = credits
		} else if isIMDbInfo(s) {
			p.Rating = &Rating{
				System: "IMDb",
				Value:  strings.Split(s, "IMDb ocena: ")[1],
			}
		} else if isDescriptionInfo(s) {
			p.Description = &Description{
				Lang:  "sl",
				Value: html.UnescapeString(s),
			}
		}
	}
}

func isEpisodeInfo(text string) bool {
	return strings.Contains(text, "epizoda")
}

func isCountryInfo(text string) bool {
	for _, country := range []string{"ZDA", "NEW ZEALAND", "Nemčija", "Hrvaška"} {
		if strings.Contains(text, country) {
			return true
		}
	}
	return false
}

func isCastInfo(text string) bool {
	return strings.Contains(text, "I: ") || strings.Contains(text, "R: ")
}

func isDescriptionInfo(text string) bool {
	return len(text) > 20
}

func isIMDbInfo(text string) bool {
	return strings.Contains(text, "IMDb ocena: ")
}
```

Key changes from the original:
- Removed all `colly` imports and collector setup
- `runTvSporedi` now calls `fetchRenderedHTML` for the main page and each channel page
- Channel links extracted with `goquery.Find()` instead of `c.OnHTML()`
- Programme divs found with `chDoc.Find("div.schedule > div:not(.clear)")` — equivalent to Colly's `div[class=schedule] > div[class!=clear]`
- All `*html.Node` helper functions (`findFullPrevSibling`, `timeOffset`, `getDesc`, `processData`, etc.) preserved unchanged
- Channel URL constructed directly instead of via `e.Request.AbsoluteURL()`
- Replaced `ioutil.WriteFile` with `os.WriteFile`

- [ ] **Step 2: Verify it compiles (together with browser.go)**

Run: `go build ./...`
Expected: Only errors from `siol.go` (still uses colly). No errors from `tvsporedi.go` or `browser.go`.

- [ ] **Step 3: Commit**

```bash
git add tvsporedi.go
git commit -m "feat: refactor tvsporedi scraper from colly to chromedp + goquery"
```

---

### Task 4: Rewrite siol.go with full implementation using chromedp + goquery

**Files:**
- Modify: `siol.go`

- [ ] **Step 1: Rewrite siol.go**

Replace the entire content of `siol.go` with:

```go
package main

import (
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func getTodayDatePlusString(plus int) string {
	y, m, d := time.Now().Add(time.Hour * 24 * time.Duration(plus)).Date()

	returnString := fmt.Sprintf("/datum/%d", y)

	if m < 10 {
		returnString += "0"
	}

	returnString = fmt.Sprintf("%s%d", returnString, m)

	if d < 10 {
		returnString += "0"
	}

	returnString = fmt.Sprintf("%s%d", returnString, d)

	return returnString
}

func getTodayDatePlusCompact(plus int) string {
	y, m, d := time.Now().Add(time.Hour * 24 * time.Duration(plus)).Date()
	return fmt.Sprintf("%d%02d%02d", y, m, d)
}

func runSiol(outPath string) {
	tv := &Tv{
		GeneratorInfoName: "peter",
		GeneratorInfoUrl:  "peter",
	}

	// Fetch channels page
	channelsHTML, err := fetchRenderedHTML("https://tv-spored.siol.net/kanali")
	if err != nil {
		log.Fatalf("Failed to fetch siol channels page: %s", err)
	}

	channelsDoc, err := goquery.NewDocumentFromReader(strings.NewReader(channelsHTML))
	if err != nil {
		log.Fatalf("Failed to parse siol channels page: %s", err)
	}

	// Extract channel links — each channel is an <a> with href like /kanal/{id}
	type siolChannel struct {
		id   string
		name string
	}
	var channels []siolChannel

	channelsDoc.Find("a[href^='/kanal/']").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}
		parts := strings.Split(href, "/")
		if len(parts) < 3 {
			return
		}
		chanID := parts[2]
		if chanID == "" {
			return
		}

		// Skip duplicates
		for _, existing := range channels {
			if existing.id == chanID {
				return
			}
		}

		// Use the alt text from the channel image, or fall back to the ID
		chanName := chanID
		img := s.Find("img")
		if alt, exists := img.Attr("alt"); exists && alt != "" {
			chanName = alt
		}

		channels = append(channels, siolChannel{id: chanID, name: chanName})
	})

	for _, ch := range channels {
		fmt.Println(ch.name)

		tv.Channel = append(tv.Channel, &Channel{
			Id: ch.id,
			DisplayName: &DisplayName{
				Lang:  "sl",
				Value: ch.name,
			},
		})

		// Fetch schedule for today + next 2 days
		for dayOffset := 0; dayOffset < 3; dayOffset++ {
			dateCompact := getTodayDatePlusCompact(dayOffset)
			scheduleURL := fmt.Sprintf("https://tv-spored.siol.net/kanal/%s/datum/%s", ch.id, dateCompact)

			scheduleHTML, err := fetchRenderedHTML(scheduleURL)
			if err != nil {
				fmt.Printf("Failed to fetch schedule for %s on %s: %s\n", ch.id, dateCompact, err)
				continue
			}

			schedDoc, err := goquery.NewDocumentFromReader(strings.NewReader(scheduleHTML))
			if err != nil {
				fmt.Printf("Failed to parse schedule for %s on %s: %s\n", ch.id, dateCompact, err)
				continue
			}

			var previousProgramme *Programme

			// Each programme is an <a> linking to the show detail page
			// href format: /kanal/{id}/oddaja/{slug}/{showId}/datum/{date}
			schedDoc.Find("a[href*='/oddaja/']").Each(func(_ int, s *goquery.Selection) {
				href, exists := s.Attr("href")
				if !exists {
					return
				}

				// Extract programme title and time from the link text
				text := strings.TrimSpace(s.Text())
				if text == "" {
					return
				}

				// The link text contains time and title, e.g. "7.00 Dobro jutro Razvedrilni 60 min"
				// Parse the time (first token in format H.MM or HH.MM)
				parts := strings.Fields(text)
				if len(parts) < 2 {
					return
				}

				timeStr := parts[0]
				if !strings.Contains(timeStr, ".") {
					return
				}

				// Convert "7.00" format to "07:00" for parseTime
				timeParts := strings.Split(timeStr, ".")
				if len(timeParts) != 2 {
					return
				}
				hour := fmt.Sprintf("%s:%s", timeParts[0], timeParts[1])
				if len(timeParts[0]) == 1 {
					hour = fmt.Sprintf("0%s:%s", timeParts[0], timeParts[1])
				}

				// Title is everything after the time, minus trailing category/duration
				title := strings.Join(parts[1:], " ")
				// Try to strip trailing " XX min" pattern
				for i := len(parts) - 1; i >= 2; i-- {
					if parts[i] == "min" && i > 0 {
						title = strings.Join(parts[1:i-1], " ")
						break
					}
				}
				if title == "" {
					return
				}

				y, m, d := time.Now().Add(time.Hour * 24 * time.Duration(dayOffset)).Date()

				p := &Programme{
					Channel: ch.id,
					Start:   parseTime(y, m, d, hour),
					Title: &Title{
						Lang:  "sl",
						Value: title,
					},
				}

				// Fetch show detail page for description
				showURL := "https://tv-spored.siol.net" + href
				showHTML, err := fetchRenderedHTML(showURL)
				if err == nil {
					showDoc, err := goquery.NewDocumentFromReader(strings.NewReader(showHTML))
					if err == nil {
						parseSiolShowDetail(showDoc, p)
					}
				}

				if previousProgramme != nil {
					previousProgramme.Stop = p.Start
				}
				previousProgramme = p
				tv.Programme = append(tv.Programme, p)
			})
		}
	}

	out, err := xml.MarshalIndent(tv, " ", "  ")
	if err != nil {
		fmt.Println(err)
	}

	err = os.WriteFile(outPath+"epg_siol.xml", out, 0644)
	if err != nil {
		log.Fatalf("Error writing to file: %s", err)
	}
}

func parseSiolShowDetail(doc *goquery.Document, p *Programme) {
	// Description: look for paragraph text in the main content area
	desc := strings.TrimSpace(doc.Find("p").FilterFunction(func(_ int, s *goquery.Selection) bool {
		text := strings.TrimSpace(s.Text())
		return len(text) > 20
	}).First().Text())
	if desc != "" {
		p.Description = &Description{
			Lang:  "sl",
			Value: desc,
		}
	}

	// Category: look for genre labels
	doc.Find("a[href*='/zvrst/']").Each(func(i int, s *goquery.Selection) {
		if i == 0 {
			p.Category = &Category{
				Lang:  "sl",
				Value: strings.TrimSpace(s.Text()),
			}
		}
	})
}

func parseShowTime(t, date string) (string, string) {
	times := strings.Split(t, "-")

	dates := strings.Split(date, ". ")

	start := dates[2] + dates[1] + dates[0] + strings.ReplaceAll(strings.TrimSpace(times[0]), ":", "") + "00 +0200"
	end := dates[2] + dates[1] + dates[0] + strings.ReplaceAll(strings.TrimSpace(times[1]), ":", "") + "00 +0200"

	return start, end
}
```

Key changes from the original:
- Removed all `colly` imports and collector setup
- Removed the `akanal`-only filter — now scrapes all channels
- Added `getTodayDatePlusCompact` helper for YYYYMMDD format URLs
- Channel discovery uses `a[href^='/kanal/']` selector, deduplicates, extracts name from `img[alt]`
- For each channel, fetches schedule pages for today + 2 days
- Parses programme entries from `a[href*='/oddaja/']` links
- Extracts time (H.MM format) and title from link text
- Fetches each show's detail page for description and category via `parseSiolShowDetail`
- `parseSiolShowDetail` extracts description from long `<p>` text and category from genre links
- Replaced `ioutil.WriteFile` with `os.WriteFile`

- [ ] **Step 2: Build the full project**

Run: `go build ./...`
Expected: No compilation errors. All files should compile cleanly now.

- [ ] **Step 3: Commit**

```bash
git add siol.go
git commit -m "feat: rewrite siol scraper with chromedp + goquery, full implementation"
```

---

### Task 5: Clean up deprecated ioutil usage in t2.go and t2_test.go

**Files:**
- Modify: `t2.go`
- Modify: `t2_test.go`

Since we bumped to Go 1.21, replace deprecated `ioutil` calls in the remaining files.

- [ ] **Step 1: Update t2.go imports and calls**

In `t2.go`, replace:
- `"io/ioutil"` → `"io"` and `"os"`
- `ioutil.ReadAll(resp.Body)` → `io.ReadAll(resp.Body)`
- `ioutil.WriteFile(...)` → `os.WriteFile(...)`

- [ ] **Step 2: Update t2_test.go imports and calls**

In `t2_test.go`, replace:
- `"io/ioutil"` → `"io"`
- `ioutil.ReadAll(resp.Body)` → `io.ReadAll(resp.Body)`

- [ ] **Step 3: Verify build and tests**

Run: `go build ./...`
Expected: No errors.

Run: `go test ./...`
Expected: Tests pass (or skip if they require network access).

- [ ] **Step 4: Commit**

```bash
git add t2.go t2_test.go
git commit -m "chore: replace deprecated ioutil in t2.go and t2_test.go"
```

---

### Task 6: Run go mod tidy and verify final build

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Tidy modules**

```bash
go mod tidy
```

- [ ] **Step 2: Verify clean build**

Run: `go build -o epGrab ./...`
Expected: Binary `epGrab` created with no errors.

- [ ] **Step 3: Verify no colly references remain**

Run: `grep -r "colly" *.go`
Expected: No matches.

Run: `grep "colly" go.mod`
Expected: No matches.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: final go mod tidy, remove all colly transitive deps"
```

---

### Task 7: Manual smoke test

- [ ] **Step 1: Test tvsporedi scraper**

Run: `go run . -scraper tvsporedi -out ./test_output/`
Expected: Creates `test_output/epg_tvsporedi.xml` with channel and programme data. Should not get 403 Forbidden.

- [ ] **Step 2: Test siol scraper**

Run: `go run . -scraper siol -out ./test_output/`
Expected: Creates `test_output/epg_siol.xml` with channel and programme data.

- [ ] **Step 3: Test t2 scraper still works**

Run: `go run . -scraper t2 -out ./test_output/`
Expected: Creates `test_output/epg_t2.xml` — unchanged behavior.

- [ ] **Step 4: Test a1 scraper still works**

Run: `go run . -scraper a1 -out ./test_output/`
Expected: Creates `test_output/epg_a1.xml` — unchanged behavior.

- [ ] **Step 5: Clean up test output**

```bash
rm -rf test_output/
```

- [ ] **Step 6: Fix any issues found during smoke testing and commit fixes**

If any scraper fails, diagnose and fix. The siol scraper in particular may need selector adjustments based on the live site structure — the selectors in Task 4 are based on the current site but the HTML structure may have details not visible in the web fetch. Adjust selectors as needed and recommit.
