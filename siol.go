package main

import (
	"encoding/xml"
	"fmt"
	"github.com/gocolly/colly"
	"io/ioutil"
	"log"
	"strings"
	"time"
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

func runSiol(outPath string) {
	c := colly.NewCollector(
		colly.AllowedDomains("tv-spored.siol.net", "www.tv-spored.siol.net"),
	)

	channelCollector := c.Clone()
	showCollector := c.Clone()

	todayDateString := getTodayDatePlusString(0)

	tv := &Tv{
		GeneratorInfoName: "peter",
		GeneratorInfoUrl:  "peter",
	}

	c.OnHTML("div[class=table-list-rows] a[href]", func(e *colly.HTMLElement) {
		if e.Attr("href") != "/kanal/akanal" {
			return
		}
		//
		//fmt.Println(e.Attr("href"))

		channelURL := e.Request.AbsoluteURL(e.Attr("href") + todayDateString)

		chanId := strings.Split(e.Attr("href"), "/")[2]
		chanName := e.ChildText("div.col-11")

		tv.Channel = append(tv.Channel, &Channel{
			Id: chanId,
			DisplayName: &DisplayName{
				Lang:  "sl",
				Value: chanName,
			},
		})

		//fmt.Println(channelURL)
		err := channelCollector.Visit(channelURL)
		if err != nil {
			fmt.Println(err)
		}
	})

	channelCollector.OnHTML("div[class=table-list-rows] a[class=row]", func(e *colly.HTMLElement) {
		//fmt.Println(e.Attr("href"))
		showURL := e.Request.AbsoluteURL(e.Attr("href"))

		if e.Attr("href") != "/kanal/akanal/oddaja/vsi-zupanovi-mozje/659149465/datum/20230312" {
			return
		}
		//
		//fmt.Println(showURL)
		err := showCollector.Visit(showURL)
		if err != nil {
			fmt.Println(err)
		}
	})

	showCollector.OnHTML("div[class=text-text]", func(e *colly.HTMLElement) {
		fmt.Println(e.Text)
		//fmt.Println(e.ChildText("div.date"))
		//fmt.Println(e.ChildText("div.time"))
		//fmt.Println(e.ChildText("h2.inline-heading span"))
		//fmt.Println(e.ChildText("p.event-meta"))
		//e.ForEach("p.content", func(i int, e *colly.HTMLElement) {
		//	fmt.Println(i, e.Text)
		//})

		//chanName := strings.Split(e.Request.URL.Path, "/")[2]
		//
		//var name string
		////var origName string
		//
		//e.ForEach("h2.inline-heading span", func(i int, e *colly.HTMLElement) {
		//	if i == 0 {
		//		name = e.Text
		//	}
		//	//if i == 1 {
		//	//	origName = e.Text
		//	//}
		//})
		//
		//start, stop := parseShowTime(e.ChildText("div.time"), e.ChildText("div.date"))
		//
		//p := &Programme{
		//	Channel: chanName,
		//	Start:   start,
		//	Stop:    stop,
		//	//Description: &Description{
		//	//	Lang:  "sl",
		//	//	Value: rawData,
		//	//},
		//	Title: &Title{
		//		Lang:  "sl",
		//		Value: name,
		//	},
		//}
		//tv.Programme = append(tv.Programme, p)
	})

	err := c.Visit("https://tv-spored.siol.net/kanali")
	if err != nil {
		fmt.Println(err)
	}

	out, err := xml.MarshalIndent(tv, " ", "  ")
	if err != nil {
		fmt.Println(err)
	}

	err = ioutil.WriteFile(outPath+"epg_siol.xml", out, 0644)
	if err != nil {
		log.Fatalf("Error writing to ffile: %s", err)
	}
}

func parseShowTime(t, date string) (string, string) {
	times := strings.Split(t, "-")

	dates := strings.Split(date, ". ")

	start := dates[2] + dates[1] + dates[0] + strings.ReplaceAll(strings.TrimSpace(times[0]), ":", "") + "00 +0200"
	end := dates[2] + dates[1] + dates[0] + strings.ReplaceAll(strings.TrimSpace(times[1]), ":", "") + "00 +0200"

	return start, end
}
