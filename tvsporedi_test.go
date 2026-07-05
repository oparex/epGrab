package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func mockFlaresolverrServer(t *testing.T, channelsHTML, scheduleHTML string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req fsRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		switch req.Cmd {
		case "sessions.create":
			json.NewEncoder(w).Encode(fsResponse{Status: "ok", Session: "test-sess"})
		case "sessions.destroy":
			json.NewEncoder(w).Encode(fsResponse{Status: "ok"})
		case "request.get":
			body := scheduleHTML
			if req.URL == "https://www.tvsporedi.si/" {
				body = channelsHTML
			}
			json.NewEncoder(w).Encode(fsResponse{
				Status:   "ok",
				Solution: fsSolution{Status: 200, Response: body},
			})
		}
	}))
}

func TestRunTvSporedi_writesXML(t *testing.T) {
	channelsHTML, err := os.ReadFile("testdata/tvsporedi_channels.html")
	require.NoError(t, err)
	scheduleHTML, err := os.ReadFile("testdata/tvsporedi_schedule.html")
	require.NoError(t, err)

	srv := mockFlaresolverrServer(t, string(channelsHTML), string(scheduleHTML))
	defer srv.Close()

	dir := t.TempDir()
	runTvSporedi(dir+"/", srv.URL, false)

	xmlData, err := os.ReadFile(dir + "/epg_tvsporedi.xml")
	require.NoError(t, err)
	xml := string(xmlData)
	assert.Contains(t, xml, `<channel id="SLO1">`)
	assert.Contains(t, xml, `<channel id="SLO2">`)
	assert.Contains(t, xml, `<channel id="POP_TV">`)
	assert.Contains(t, xml, "Jutarnji program")
	assert.Contains(t, xml, "Oddaja za otroke")
}

func TestRunTvSporedi_dryRun_noFile(t *testing.T) {
	channelsHTML, err := os.ReadFile("testdata/tvsporedi_channels.html")
	require.NoError(t, err)
	scheduleHTML, err := os.ReadFile("testdata/tvsporedi_schedule.html")
	require.NoError(t, err)

	srv := mockFlaresolverrServer(t, string(channelsHTML), string(scheduleHTML))
	defer srv.Close()

	dir := t.TempDir()
	runTvSporedi(dir+"/", srv.URL, true)

	_, err = os.Stat(dir + "/epg_tvsporedi.xml")
	assert.True(t, os.IsNotExist(err), "dry run must not write epg_tvsporedi.xml")
}
