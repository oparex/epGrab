package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func sporediBase(t *testing.T) *url.URL {
	t.Helper()
	u, err := url.Parse(tvSporediBase)
	require.NoError(t, err)
	return u
}

func TestParseTvSporediChannels(t *testing.T) {
	doc := loadTvSporediDoc(t, "testdata/tvsporedi_channels.html")
	channels := parseTvSporediChannels(doc, sporediBase(t))
	assert.Equal(t, []tvSporediChannel{
		{id: "SLO 1 HD", scheduleURL: "https://www.tvsporedi.si/spored.php?urr=094408&id=SLO+1+HD"},
		{id: "POP HD", scheduleURL: "https://www.tvsporedi.si/spored.php?urr=094409&id=POP+HD"},
	}, channels)
}

func TestParseTvSporediChannels_dedup(t *testing.T) {
	doc := loadTvSporediDoc(t, "testdata/tvsporedi_channels.html")
	channels := parseTvSporediChannels(doc, sporediBase(t))
	seen := make(map[string]int)
	for _, ch := range channels {
		seen[ch.id]++
	}
	for id, count := range seen {
		assert.Equal(t, 1, count, "channel %s appears more than once", id)
	}
}

func TestParseTvSporediSchedule(t *testing.T) {
	doc := loadTvSporediDoc(t, "testdata/tvsporedi_schedule.html")
	programmes := parseTvSporediSchedule(doc, "SLO1", 2026, time.July, 5)

	require.Len(t, programmes, 4, "2 from #a + 1 from #b + 1 from #c; bad-time row excluded")

	p0 := programmes[0]
	assert.Equal(t, "SLO1", p0.Channel)
	assert.Equal(t, "20260705060000 +0200", p0.Start)
	assert.Equal(t, "Jutarnji program", p0.Title.Value)
	assert.Equal(t, "sl", p0.Title.Lang)
	require.NotNil(t, p0.Description)
	assert.Equal(t, "Jutarnji program opis.", p0.Description.Value)
	assert.Equal(t, "sl", p0.Description.Lang)
	require.NotNil(t, p0.Country)
	assert.Equal(t, "Slovenija", p0.Country.Value)
	assert.Nil(t, p0.EpisodeNumber)

	p1 := programmes[1]
	assert.Equal(t, "20260705083000 +0200", p1.Start)
	assert.Equal(t, "Oddaja za otroke", p1.Title.Value)
	require.NotNil(t, p1.Description)
	assert.Equal(t, "Otroška oddaja opis.", p1.Description.Value)
	require.NotNil(t, p1.EpisodeNumber)
	assert.Equal(t, "E05", p1.EpisodeNumber.Value)
	assert.Equal(t, "onscreen", p1.EpisodeNumber.System)
	assert.Nil(t, p1.Country)

	p2 := programmes[2]
	assert.Equal(t, "20260706090000 +0200", p2.Start)
	assert.Equal(t, "Jutri oddaja", p2.Title.Value)

	p3 := programmes[3]
	assert.Equal(t, "20260707100000 +0200", p3.Start)
	assert.Equal(t, "Pojutrišnjem oddaja", p3.Title.Value)
}

func TestParseTvSporediMeta(t *testing.T) {
	cases := []struct {
		name        string
		html        string
		wantDesc    string
		wantOrig    string
		wantEpisode string
		wantCountry string
	}{
		{
			name:        "no desc div",
			html:        `<div><div class="time">06:00</div><div class="prog">Title</div></div>`,
			wantDesc:    "",
			wantOrig:    "",
			wantEpisode: "",
			wantCountry: "",
		},
		{
			name:        "simple bare-text desc (sports entries)",
			html:        `<div><div class="prog"><a href="#">Odbojka: Bolgarija - Nemčija</a></div><div class="toggle-item31563a desc">Liga narodov (ž)</div></div>`,
			wantDesc:    "Liga narodov (ž)",
			wantOrig:    "",
			wantEpisode: "",
			wantCountry: "",
		},
		{
			name:        "desc with all fields",
			html:        `<div><div class="toggle-item1a desc" style="display:none;"><br>Original naslov: Orig<br>Epizoda: E03/06<br>Država: Nemčija<br> <p>Opis oddaje.</p><br></div></div>`,
			wantDesc:    "Opis oddaje.",
			wantOrig:    "Orig",
			wantEpisode: "E03/06",
			wantCountry: "Nemčija",
		},
		{
			name:        "desc with description only",
			html:        `<div><div class="toggle-item2a desc" style="display:none;"><br>Original naslov: Show<br> <p>Samo opis.</p><br></div></div>`,
			wantDesc:    "Samo opis.",
			wantOrig:    "Show",
			wantEpisode: "",
			wantCountry: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(tc.html))
			require.NoError(t, err)
			s := doc.Find("div").First()
			desc, orig, ep, country := parseTvSporediMeta(s)
			assert.Equal(t, tc.wantDesc, desc)
			assert.Equal(t, tc.wantOrig, orig)
			assert.Equal(t, tc.wantEpisode, ep)
			assert.Equal(t, tc.wantCountry, country)
		})
	}
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
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("mockFlaresolverrServer: decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		switch req.Cmd {
		case "sessions.create":
			json.NewEncoder(w).Encode(fsResponse{Status: "ok", Session: "test-sess"})
		case "sessions.destroy":
			json.NewEncoder(w).Encode(fsResponse{Status: "ok"})
		case "request.get":
			body := scheduleHTML
			if req.URL == tvSporediBase {
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
	assert.Contains(t, xml, `<channel id="SLO 1 HD">`)
	assert.Contains(t, xml, `<channel id="POP HD">`)
	assert.Contains(t, xml, "Jutarnji program")
	assert.Contains(t, xml, "Oddaja za otroke")
	assert.Contains(t, xml, "Jutri oddaja")
	assert.Contains(t, xml, "Pojutrišnjem oddaja")
	assert.Contains(t, xml, "Jutarnji program opis.")
	assert.Contains(t, xml, "Otroška oddaja opis.")
	assert.Contains(t, xml, `system="onscreen"`)
	assert.Contains(t, xml, "E05")
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
