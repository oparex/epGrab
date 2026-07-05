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
