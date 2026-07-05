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
