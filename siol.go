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
	// siol serves plain server-rendered HTML (nginx, no Cloudflare challenge),
	// so we fetch with a normal HTTP client — no headless browser required.
	tv := &Tv{
		GeneratorInfoName: "peter",
		GeneratorInfoUrl:  "peter",
	}

	// Fetch channels page
	channelsHTML, err := httpGetHTML("https://tv-spored.siol.net/kanali")
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

			scheduleHTML, err := httpGetHTML(scheduleURL)
			if err != nil {
				fmt.Printf("Failed to fetch schedule for %s on %s: %s\n", ch.id, dateCompact, err)
				continue
			}

			schedDoc, err := goquery.NewDocumentFromReader(strings.NewReader(scheduleHTML))
			if err != nil {
				fmt.Printf("Failed to parse schedule for %s on %s: %s\n", ch.id, dateCompact, err)
				continue
			}

			y, m, d := time.Now().Add(time.Hour * 24 * time.Duration(dayOffset)).Date()
			programmes := parseSiolSchedule(schedDoc, ch.id, y, m, d)

			var previousProgramme *Programme
			for _, sp := range programmes {
				p := sp.programme

				// Fetch show detail page for description, category and icon.
				showURL := "https://tv-spored.siol.net" + sp.href
				showHTML, err := httpGetHTML(showURL)
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
			}
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

// siolProgramme pairs a parsed Programme with the relative href of its detail
// page, which is fetched separately to fill in the description/category/icon.
type siolProgramme struct {
	programme *Programme
	href      string
}

// parseSiolSchedule extracts the programmes from a channel's daily schedule
// page. Each programme is an <a href=".../oddaja/..."> whose children are
// separate <div>s for the time, title and category — so we read those divs
// directly rather than splitting the run-together concatenated link text.
func parseSiolSchedule(doc *goquery.Document, chID string, y int, m time.Month, d int) []siolProgramme {
	var out []siolProgramme
	seen := make(map[string]bool)

	doc.Find("a[href*='/oddaja/']").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || href == "" {
			return
		}
		// The page renders responsive (desktop/mobile) duplicates of each
		// row; collapse them by detail href.
		if seen[href] {
			return
		}

		// Time: the first child <div>, e.g. "0.40".
		timeStr := strings.TrimSpace(s.Find("div").First().Text())
		hour, ok := normalizeSiolTime(timeStr)
		if !ok {
			return
		}

		// Title: the div carries a clean title="..." attribute; fall back to
		// the bold title div's text.
		title := strings.TrimSpace(s.Find("div[title]").First().AttrOr("title", ""))
		if title == "" {
			title = strings.TrimSpace(s.Find("div.font-extrabold").First().Text())
		}
		if title == "" {
			return
		}

		seen[href] = true

		p := &Programme{
			Channel: chID,
			Start:   parseTime(y, m, d, hour),
			Title: &Title{
				Lang:  "sl",
				Value: title,
			},
		}
		out = append(out, siolProgramme{programme: p, href: href})
	})

	return out
}

// normalizeSiolTime converts siol's "H.MM"/"HH.MM" time format into the
// "HH:MM" form expected by parseTime. It returns false for anything that
// isn't a time (e.g. stray links that also match the /oddaja/ selector).
func normalizeSiolTime(t string) (string, bool) {
	parts := strings.Split(t, ".")
	if len(parts) != 2 {
		return "", false
	}
	h, mnt := parts[0], parts[1]
	if len(h) == 0 || len(h) > 2 || len(mnt) != 2 {
		return "", false
	}
	for _, r := range h + mnt {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	if len(h) == 1 {
		h = "0" + h
	}
	return h + ":" + mnt, true
}

func parseSiolShowDetail(doc *goquery.Document, p *Programme) {
	// Description: the page exposes the show synopsis in the document head as
	// the og:description / description meta tags. This is far more reliable than
	// scraping <p> elements, which also contain the cookie-consent banner and
	// the schedule of other shows on the same page.
	desc, _ := doc.Find(`meta[property="og:description"]`).Attr("content")
	if strings.TrimSpace(desc) == "" {
		desc, _ = doc.Find(`meta[name="description"]`).Attr("content")
	}
	if strings.TrimSpace(desc) == "" {
		// Fallback: the visible synopsis paragraph in the main detail block.
		desc = doc.Find("p.font-bold.font-serif").First().Text()
	}
	desc = strings.TrimSpace(desc)
	if desc != "" {
		p.Description = &Description{
			Lang:  "sl",
			Value: desc,
		}
	}

	// Category: the genre sits in the <p> immediately after the show's <h1>
	// title, e.g. "Ostalo / Posebni dogodek".
	if cat := strings.TrimSpace(doc.Find("h1").First().NextFiltered("p").Text()); cat != "" {
		p.Category = &Category{
			Lang:  "sl",
			Value: cat,
		}
	}

	// Icon: the show poster, e.g. <img alt="Slika oddaje" src="...">.
	if src, ok := doc.Find(`img[alt="Slika oddaje"]`).First().Attr("src"); ok && strings.TrimSpace(src) != "" {
		p.Icon = &Icon{Src: strings.TrimSpace(src)}
	}
}

func parseShowTime(t, date string) (string, string) {
	times := strings.Split(t, "-")

	dates := strings.Split(date, ". ")

	start := dates[2] + dates[1] + dates[0] + strings.ReplaceAll(strings.TrimSpace(times[0]), ":", "") + "00 +0200"
	end := dates[2] + dates[1] + dates[0] + strings.ReplaceAll(strings.TrimSpace(times[1]), ":", "") + "00 +0200"

	return start, end
}
