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
