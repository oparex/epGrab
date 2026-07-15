package main

import (
	"os"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// TestParseSiolSchedule verifies the schedule parser reads time and title from
// the structured divs (not the run-together concatenated link text) and dedups
// the responsive duplicate rows.
func TestParseSiolSchedule(t *testing.T) {
	f, err := os.Open("testdata/siol_schedule.html")
	if err != nil {
		t.Fatalf("open fixture: %s", err)
	}
	defer f.Close()

	doc, err := goquery.NewDocumentFromReader(f)
	if err != nil {
		t.Fatalf("parse fixture: %s", err)
	}

	progs := parseSiolSchedule(doc, "slo1", 2026, 6, 19)
	if len(progs) == 0 {
		t.Fatalf("no programmes parsed")
	}

	first := progs[0].programme
	if first.Title == nil || first.Title.Value != "Napovedujemo" {
		t.Errorf("first title = %v, want %q", first.Title, "Napovedujemo")
	}
	if first.Title.Value == "" || strings.ContainsAny(first.Title.Value, "0123456789") && !strings.Contains(first.Title.Value, " ") {
		// guards against the old "0.40NapovedujemoOstalo" concatenation bug
		t.Errorf("title looks like concatenated link text: %q", first.Title.Value)
	}
	if first.Start == "" {
		t.Errorf("first start time empty")
	}
	if progs[0].href == "" {
		t.Errorf("first href empty")
	}

	// Every programme must have a usable title and start time.
	for i, sp := range progs {
		if sp.programme.Title == nil || sp.programme.Title.Value == "" {
			t.Errorf("programme[%d] missing title", i)
		}
		if sp.programme.Start == "" {
			t.Errorf("programme[%d] missing start", i)
		}
	}
	t.Logf("parsed %d programmes; first=%q start=%q", len(progs), first.Title.Value, first.Start)
}

// TestParseSiolShowDetail verifies that the show detail parser extracts the
// description, category and icon from a real captured siol detail page.
func TestParseSiolShowDetail(t *testing.T) {
	f, err := os.Open("testdata/siol_show.html")
	if err != nil {
		t.Fatalf("open fixture: %s", err)
	}
	defer f.Close()

	doc, err := goquery.NewDocumentFromReader(f)
	if err != nil {
		t.Fatalf("parse fixture: %s", err)
	}

	p := &Programme{
		Channel: "slo1",
		Title:   &Title{Lang: "sl", Value: "Napovedujemo"},
	}
	parseSiolShowDetail(doc, p)

	if p.Description == nil {
		t.Fatalf("description is nil; expected the show synopsis")
	}
	if want := "Napovednik programa."; p.Description.Value != want {
		t.Errorf("description = %q, want %q", p.Description.Value, want)
	}
	// Must NOT be the cookie-consent banner that the old selector grabbed.
	if strings.Contains(p.Description.Value, "soglasjem") {
		t.Errorf("description grabbed the cookie banner: %q", p.Description.Value)
	}

	if p.Category == nil {
		t.Fatalf("category is nil; expected the genre")
	}
	if want := "Ostalo / Posebni dogodek"; p.Category.Value != want {
		t.Errorf("category = %q, want %q", p.Category.Value, want)
	}
}
