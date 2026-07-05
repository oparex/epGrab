package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadTvSporediDoc(t *testing.T, path string) *goquery.Document {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(b)))
	require.NoError(t, err)
	return doc
}

func TestParseTvSporediChannels(t *testing.T) {
	doc := loadTvSporediDoc(t, "testdata/tvsporedi_channels.html")
	channels := parseTvSporediChannels(doc)
	assert.Equal(t, []string{"SLO1", "SLO2", "POP_TV"}, channels)
}

func TestParseTvSporediChannels_dedup(t *testing.T) {
	doc := loadTvSporediDoc(t, "testdata/tvsporedi_channels.html")
	channels := parseTvSporediChannels(doc)
	seen := make(map[string]int)
	for _, ch := range channels {
		seen[ch]++
	}
	for id, count := range seen {
		assert.Equal(t, 1, count, "channel %s appears more than once", id)
	}
}

func TestParseTvSporediSchedule(t *testing.T) {
	doc := loadTvSporediDoc(t, "testdata/tvsporedi_schedule.html")
	programmes := parseTvSporediSchedule(doc, "SLO1", 2026, time.July, 5)

	require.Len(t, programmes, 2, "bad-time row and tomorrow row must be excluded")

	assert.Equal(t, "SLO1", programmes[0].Channel)
	assert.Equal(t, "20260705060000 +0200", programmes[0].Start)
	assert.Equal(t, "Jutarnji program", programmes[0].Title.Value)
	assert.Equal(t, "sl", programmes[0].Title.Lang)

	assert.Equal(t, "20260705083000 +0200", programmes[1].Start)
	assert.Equal(t, "Oddaja za otroke", programmes[1].Title.Value)
}

func TestIsValidTvSporediTime(t *testing.T) {
	assert.True(t, isValidTvSporediTime("06:00"))
	assert.True(t, isValidTvSporediTime("23:59"))
	assert.False(t, isValidTvSporediTime("bad-time"))
	assert.False(t, isValidTvSporediTime("6:00"))
	assert.False(t, isValidTvSporediTime(""))
	assert.False(t, isValidTvSporediTime("06:0"))
}
