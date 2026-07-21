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

const tvSporediBase = "https://www.tvsporedi.si/spored.php"

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

	baseURL, _ := url.Parse(tvSporediBase)

	chanListHTML, err := flaresolverrGetHTML(tvSporediBase, endpoint, sessionID)
	if err != nil {
		fmt.Printf("tvsporedi: fetch channel list: %s\n", err)
		return
	}

	chanListDoc, err := goquery.NewDocumentFromReader(strings.NewReader(chanListHTML))
	if err != nil {
		fmt.Printf("tvsporedi: parse channel list: %s\n", err)
		return
	}

	channels := parseTvSporediChannels(chanListDoc, baseURL)
	if len(channels) == 0 {
		fmt.Println("tvsporedi: no channels found — selectors may need updating for current site HTML")
		return
	}

	tv := &Tv{
		GeneratorInfoName: "peter",
		GeneratorInfoUrl:  "peter",
	}

	y, m, d := time.Now().Date()

	for _, ch := range channels {
		fmt.Println(ch.id)

		tv.Channel = append(tv.Channel, &Channel{
			Id: ch.id,
			DisplayName: &DisplayName{
				Lang:  "sl",
				Value: ch.id,
			},
		})

		schedHTML, err := flaresolverrGetHTML(ch.scheduleURL, endpoint, sessionID)
		if err != nil {
			fmt.Printf("tvsporedi: fetch schedule for %s: %s\n", ch.id, err)
			continue
		}

		schedDoc, err := goquery.NewDocumentFromReader(strings.NewReader(schedHTML))
		if err != nil {
			fmt.Printf("tvsporedi: parse schedule for %s: %s\n", ch.id, err)
			continue
		}

		programmes := parseTvSporediSchedule(schedDoc, ch.id, y, m, d)

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

// tvSporediChannel pairs a channel's display ID with the full URL of its
// schedule page (including the site's urr= token which cannot be computed
// independently and must be taken from the channel-list link).
type tvSporediChannel struct {
	id          string
	scheduleURL string
}

// parseTvSporediChannels extracts unique channels from spored.php's
// #left-navigation. Links there are relative: href="?urr=XXXX&id=CHANNEL+NAME".
// baseURL (https://www.tvsporedi.si/spored.php) is used to resolve them into
// absolute schedule URLs.
func parseTvSporediChannels(doc *goquery.Document, baseURL *url.URL) []tvSporediChannel {
	var channels []tvSporediChannel
	seen := make(map[string]bool)

	doc.Find(`#left-navigation a[href*="id="]`).Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		rel, err := url.Parse(href)
		if err != nil {
			return
		}
		chanID := rel.Query().Get("id")
		if chanID == "" || seen[chanID] {
			return
		}
		seen[chanID] = true
		channels = append(channels, tvSporediChannel{
			id:          chanID,
			scheduleURL: baseURL.ResolveReference(rel).String(),
		})
	})
	return channels
}

// parseTvSporediSchedule extracts programmes for today, tomorrow, and the day
// after from a channel schedule page. The site puts each day in a separate
// div: #a = today, #b = tomorrow, #c = day after tomorrow.
func parseTvSporediSchedule(doc *goquery.Document, chanID string, y int, m time.Month, d int) []*Programme {
	var out []*Programme

	base := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	for i, sel := range []string{"#a", "#b", "#c"} {
		day := base.AddDate(0, 0, i)
		dy, dm, dd := day.Date()

		doc.Find(sel+" .schedule > div").Each(func(_ int, s *goquery.Selection) {
			if s.HasClass("clear") {
				return
			}
			timeStr := strings.TrimSpace(s.Find(".time").First().Text())
			title := strings.TrimSpace(s.Find(".prog").First().Text())
			if !isValidTvSporediTime(timeStr) || title == "" {
				return
			}

			prog := &Programme{
				Channel: chanID,
				Start:   parseTime(dy, dm, dd, timeStr),
				Title: &Title{
					Lang:  "sl",
					Value: title,
				},
			}

			description, _, episode, country := parseTvSporediMeta(s)
			if description != "" {
				prog.Description = &Description{Lang: "sl", Value: description}
			}
			if episode != "" {
				prog.EpisodeNumber = &EpisodeNumber{System: "onscreen", Value: episode}
			}
			if country != "" {
				prog.Country = &Country{Value: country}
			}

			out = append(out, prog)
		})
	}
	return out
}

// parseTvSporediMeta extracts the hidden metadata from a show's .desc element.
// The div contains <br>-separated key: value lines before a <p> description.
// origTitle may be empty when the site omits it (identical to display title).
func parseTvSporediMeta(s *goquery.Selection) (description, origTitle, episode, country string) {
	desc := s.Find(".desc").First()
	if desc.Length() == 0 {
		return
	}

	description = strings.TrimSpace(desc.Find("p").First().Text())
	if description == "" {
		// Simple one-line desc: bare text with no <p> or <br> structure.
		description = strings.TrimSpace(desc.Text())
	}

	// Extract the portion of inner HTML before the <p> block and split on <br>.
	// goquery serializes void <br> elements as <br/>, so normalise before splitting.
	rawHTML, _ := desc.Html()
	rawHTML = strings.ReplaceAll(rawHTML, "<br/>", "<br>")
	metaPart := rawHTML
	if idx := strings.Index(rawHTML, "<p>"); idx >= 0 {
		metaPart = rawHTML[:idx]
	}
	for _, line := range strings.Split(metaPart, "<br>") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Original naslov:"):
			origTitle = strings.TrimSpace(strings.TrimPrefix(line, "Original naslov:"))
		case strings.HasPrefix(line, "Epizoda:"):
			episode = strings.TrimSpace(strings.TrimPrefix(line, "Epizoda:"))
		case strings.HasPrefix(line, "Država:"):
			country = strings.TrimSpace(strings.TrimPrefix(line, "Država:"))
		}
	}
	return
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
