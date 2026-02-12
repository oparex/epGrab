package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly"
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
	// Compare times properly, not lexicographically
	lastTime := lastNode.FirstChild.FirstChild.Data
	currentTime := currentNode.FirstChild.FirstChild.Data
	if compareTimeStrings(lastTime, currentTime) > 0 {
		return 1
	}
	return timeOffset(lastNode)
}

// compareTimeStrings compares two time strings in "HH:MM" or "H:MM" format
// Returns: >0 if time1 > time2, 0 if equal, <0 if time1 < time2
func compareTimeStrings(time1, time2 string) int {
	parts1 := strings.Split(time1, ":")
	parts2 := strings.Split(time2, ":")
	
	if len(parts1) != 2 || len(parts2) != 2 {
		// For unexpected formats, treat as equal to avoid incorrect midnight detection
		// This is safer than lexicographic comparison which has the midnight bug
		log.Printf("Warning: Unexpected time format for comparison: %q vs %q", time1, time2)
		return 0
	}
	
	hour1, err1 := strconv.Atoi(parts1[0])
	minute1, err2 := strconv.Atoi(parts1[1])
	hour2, err3 := strconv.Atoi(parts2[0])
	minute2, err4 := strconv.Atoi(parts2[1])
	
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		// For parsing errors, treat as equal to avoid incorrect midnight detection
		log.Printf("Warning: Failed to parse time components: %q vs %q", time1, time2)
		return 0
	}
	
	// Compare hours first, then minutes
	if hour1 != hour2 {
		return hour1 - hour2
	}
	return minute1 - minute2
}

func runTvSporedi(outPath string) {
	// Instantiate default collector
	c := colly.NewCollector(
		colly.AllowedDomains("tvsporedi.si", "www.tvsporedi.si"),
	)
	detailCollector := c.Clone()

	tv := &Tv{
		GeneratorInfoName: "peter",
		GeneratorInfoUrl:  "peter",
	}

	c.OnHTML("div[id=left-navigation] a[href]", func(e *colly.HTMLElement) {
		params, err := url.ParseQuery(e.Attr("href"))
		if err != nil {
			log.Fatalf("Problem decoding url %s: %s", e.Attr("href"), err)
		}
		if len(params["id"]) == 0 {
			log.Fatalf("Empty id param in url %s", e.Attr("href"))
		}
		fmt.Println(params["id"][0])
		//if params["id"][0] != "HBO" {
		//	return
		//}
		tv.Channel = append(tv.Channel, &Channel{
			Id: params["id"][0],
			DisplayName: &DisplayName{
				Lang:  "sl",
				Value: params["id"][0],
			},
		})
		channelURL := e.Request.AbsoluteURL(strings.ReplaceAll(e.Attr("href"), " ", "+"))
		err = detailCollector.Visit(channelURL)
		if err != nil {
			fmt.Println(err)
		}
	})

	detailCollector.OnHTML("div[class=schedule] > div[class!=clear]", func(e *colly.HTMLElement) {
		params, err := url.ParseQuery(e.Request.URL.RawQuery)
		if err != nil {
			log.Fatalf("Problem decoding url %s: %s", e.Attr("href"), err)
		}
		if len(params["id"]) == 0 {
			log.Fatalf("Empty id param in url %s", e.Attr("href"))
		}
		hour := e.ChildText(".time")
		prog := e.ChildText(".prog")
		rawData := getDesc(e.DOM.Nodes[0].LastChild)

		if strings.Count(hour, ":") > 1 {
			return
		}

		year, month, day := time.Now().Date()
		if e.DOM.Nodes[0].Parent.Parent.Attr[0].Key == "id" {
			if e.DOM.Nodes[0].Parent.Parent.Attr[0].Val == "b" {
				day += 1
			}
			if e.DOM.Nodes[0].Parent.Parent.Attr[0].Val == "c" {
				day += 2
			}
		} else {
			return
		}
		day += timeOffset(e.DOM.Nodes[0])

		p := &Programme{
			Channel: params["id"][0],
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
		//processData(p, rawData)
		if len(tv.Programme) > 0 {
			tv.Programme[len(tv.Programme)-1].Stop = p.Start
		}
		tv.Programme = append(tv.Programme, p)
		//lastHour = hour
	})

	err := c.Visit("https://www.tvsporedi.si/spored.php")
	if err != nil {
		fmt.Println(err)
	}

	out, err := xml.MarshalIndent(tv, " ", "  ")
	if err != nil {
		fmt.Println(err)
	}

	err = ioutil.WriteFile(outPath+"epg_tvsporedi.xml", out, 0644)
	if err != nil {
		log.Fatalf("Error writing to ffile: %s", err)
	}
}

func getDesc(startNode *html.Node) string {
	currentNode := startNode.FirstChild

	//var result []string
	//for {
	//	if currentNode == nil {
	//		break
	//	}
	//	if currentNode.Data == "br" {
	//		// pass
	//	} else if currentNode.Data == "p" {
	//		result = append(result, "para:"+currentNode.FirstChild.Data)
	//	} else {
	//		if sp := strings.Split(currentNode.Data, "... "); len(sp) > 0 {
	//			result = append(result, sp...)
	//		} else if sp := strings.Split(currentNode.Data, "......"); len(sp) > 0 {
	//			result = append(result, sp...)
	//		} else {
	//			result = append(result, currentNode.Data)
	//		}
	//	}
	//	currentNode = currentNode.NextSibling
	//}
	//return result
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
