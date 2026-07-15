package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// TestSiolHTTPNoBrowser proves the full siol flow works over plain net/http
// with no headless browser at all.
func TestSiolHTTPNoBrowser(t *testing.T) {
	if testing.Short() {
		t.Skip("network test")
	}
	date := getTodayDatePlusCompact(0)
	schedURL := fmt.Sprintf("https://tv-spored.siol.net/kanal/slo1/datum/%s", date)

	schedHTML, err := httpGetHTML(schedURL)
	if err != nil {
		t.Fatalf("schedule http fetch: %s", err)
	}
	schedDoc, _ := goquery.NewDocumentFromReader(strings.NewReader(schedHTML))
	progs := parseSiolSchedule(schedDoc, "slo1", 2026, 6, 24)
	if len(progs) == 0 {
		t.Fatalf("no programmes parsed over plain HTTP")
	}

	showHTML, err := httpGetHTML("https://tv-spored.siol.net" + progs[0].href)
	if err != nil {
		t.Fatalf("detail http fetch: %s", err)
	}
	showDoc, _ := goquery.NewDocumentFromReader(strings.NewReader(showHTML))
	parseSiolShowDetail(showDoc, progs[0].programme)
	p := progs[0].programme
	if p.Description == nil || p.Description.Value == "" {
		t.Fatalf("no description over plain HTTP for %q", p.Title.Value)
	}
	t.Logf("HTTP-only: %d programmes; first=%q desc=%q", len(progs), p.Title.Value, p.Description.Value)
}

// TestSiolDumpDetail re-captures the page fixtures in testdata/ from the live
// site (channels -> schedule -> one detail page). Run with -run TestSiolDumpDetail
// when the site markup changes and the parser tests need fresh fixtures.
func TestSiolDumpDetail(t *testing.T) {
	if testing.Short() {
		t.Skip("network test")
	}

	chHTML, err := httpGetHTML("https://tv-spored.siol.net/kanali")
	if err != nil {
		t.Fatalf("channels fetch: %s", err)
	}
	os.WriteFile("testdata/siol_channels.html", []byte(chHTML), 0644)

	date := getTodayDatePlusCompact(0)
	schedHTML, err := httpGetHTML(fmt.Sprintf("https://tv-spored.siol.net/kanal/slo1/datum/%s", date))
	if err != nil {
		t.Fatalf("schedule fetch: %s", err)
	}
	os.WriteFile("testdata/siol_schedule.html", []byte(schedHTML), 0644)

	schedDoc, _ := goquery.NewDocumentFromReader(strings.NewReader(schedHTML))
	progs := parseSiolSchedule(schedDoc, "slo1", 2026, 6, 24)
	if len(progs) == 0 {
		t.Fatalf("no shows found to capture a detail fixture")
	}

	showHTML, err := httpGetHTML("https://tv-spored.siol.net" + progs[0].href)
	if err != nil {
		t.Fatalf("show fetch: %s", err)
	}
	os.WriteFile("testdata/siol_show.html", []byte(showHTML), 0644)
	t.Logf("captured fixtures: channels=%dB schedule=%dB show=%dB", len(chHTML), len(schedHTML), len(showHTML))
}
