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
