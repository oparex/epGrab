package main

// Debug helpers — dump live HTML to files for selector development.
// Run with: go test -v -run TestDumpTvSporedi -flaresolverr http://localhost:8191
// Output: testdata/tvsporedi_live_channels.html, testdata/tvsporedi_live_schedule.html

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

var fsEndpoint = flag.String("flaresolverr", "http://localhost:8191", "FlareSolverr endpoint")

func TestDumpTvSporedi(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live fetch in short mode")
	}

	sessID, err := flaresolverrCreateSession(*fsEndpoint)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer flaresolverrDestroySession(*fsEndpoint, sessID)

	// Dump full channel list from spored.php
	chanHTML, err := flaresolverrGetHTML("https://www.tvsporedi.si/spored.php", *fsEndpoint, sessID)
	if err != nil {
		t.Fatalf("fetch spored.php: %v", err)
	}
	if err := os.WriteFile("testdata/tvsporedi_live_channels.html", []byte(chanHTML), 0644); err != nil {
		t.Fatalf("write channels html: %v", err)
	}
	fmt.Println("wrote testdata/tvsporedi_live_channels.html")

	// Parse channels to get the first one, then dump its schedule page
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(chanHTML))
	if err != nil {
		t.Fatalf("parse channels: %v", err)
	}
	base, _ := url.Parse(tvSporediBase)
	channels := parseTvSporediChannels(doc, base)
	fmt.Printf("found %d channels\n", len(channels))
	for _, ch := range channels {
		fmt.Printf("  id=%q url=%s\n", ch.id, ch.scheduleURL)
	}

	// Dump SLO 1 HD schedule — rich content for inspecting show metadata HTML.
	// Fetch it directly using a channel from the parsed list if found, else hard-code.
	schedURL := "https://www.tvsporedi.si/spored.php?urr=150433&id=SLO+1+HD"
	for _, ch := range channels {
		if ch.id == "SLO 1 HD" {
			schedURL = ch.scheduleURL
			break
		}
	}
	schedHTML, err := flaresolverrGetHTML(schedURL, *fsEndpoint, sessID)
	if err != nil {
		t.Fatalf("fetch schedule: %v", err)
	}
	if err := os.WriteFile("testdata/tvsporedi_live_schedule.html", []byte(schedHTML), 0644); err != nil {
		t.Fatalf("write schedule html: %v", err)
	}
	fmt.Printf("wrote testdata/tvsporedi_live_schedule.html (%s)\n", schedURL)
}
