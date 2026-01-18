package main

import (
	"crypto/md5"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	baseUrl  = "https://tv2go.t-2.net/Catherine/api"
	version  = "9.6"
	dataType = "json"
	id       = "464830403846070"
	nonce    = "6dace810-55d5-11e3-949a-0800200c9a66"
)

type ChannelsResponse struct {
	Channels []T2Channel `json:"channels"`
}

type T2Channel struct {
	Id                int      `json:"id"`
	Name              string   `json:"name"`
	StreamResolutions []string `json:"streamResolutions"`
}

type EpgResponse struct {
	Entries []Entry
}

type Entry struct {
	ChannelId      int    `json:"channelId"`
	StartTimestamp string `json:"startTimestamp"`
	EndTimestamp   string `json:"endTimestamp"`
	Name           string `json:"name"`
	NameSingleLine string `json:"nameSingleLine"`
	Description    string `json:"description"`
	BroadcastType  string `json:"broadcastType"`
	Show           Show   `json:"show"`
}

type Show struct {
	Id               int         `json:"id"`
	Title            string      `json:"title"`
	OriginalTitle    string      `json:"originalTitle"`
	ShortDescription string      `json:"shortDescription"`
	LongDescription  string      `json:"longDescription"`
	Type             Type        `json:"type"`
	ProductionFrom   string      `json:"productionFrom"`
	Countries        []CountryT2 `json:"countries"`
	Season           Season      `json:"season"`
	Episode          Episode     `json:"episode"`
	Languages        []Language  `json:"languages"`
	Genres           []Genre     `json:"genres"`
	Ratings          []RatingT2  `json:"ratings"`
	People           []Person    `json:"people"`
}

func (s *Show) constructDescString() string {
	out := ""

	if len(s.OriginalTitle) > 0 {
		out += s.OriginalTitle + "\n"
	}
	if s.Season.Number > 0 && s.Episode.Number > 0 {
		out += strconv.Itoa(s.Season.Number) + ". sezona " + strconv.Itoa(s.Episode.Number) + ". del"
	}
	if len(s.Genres) > 0 {
		out += " | "
		for i, g := range s.Genres {
			out += g.Name
			if i < len(s.Genres)-1 {
				out += ","
			}
		}
		out += " | "
	}
	if len(s.Countries) > 0 {
		for i, c := range s.Countries {
			out += c.Name
			if i < len(s.Countries)-1 {
				out += ","
			}
		}
		out += " | "
	}
	if len(s.Languages) > 0 {
		for i, l := range s.Languages {
			out += l.Name
			if i < len(s.Languages)-1 {
				out += ","
			}
		}
		out += " | "
	}
	if len(s.ProductionFrom) > 0 {
		prodTime, err := strconv.Atoi(s.ProductionFrom)
		if err != nil {
			log.Printf("Could not convert production time %s for show %d: %s", s.ProductionFrom, s.Id, err)
		} else {
			out += strconv.Itoa(time.UnixMilli(int64(prodTime)).Year()) + "\n | "
		}
	}
	if len(s.Ratings) > 0 {
		for _, r := range s.Ratings {
			if r.ProviderId == 1 {
				out += fmt.Sprintf("IMDb ocena: %.1f / 10 | ", r.Rating)
			}
		}
	}

	if len(s.Episode.LongDescription) > 0 {
		out += s.Episode.LongDescription + "\n"
	}
	if len(s.ShortDescription) > 0 {
		out += s.ShortDescription + "\n"
	}
	out += s.LongDescription + "\n"
	out += s.constructPeopleString()
	return out
}

func (s *Show) constructPeopleString() string {
	out := ""
	if len(s.People) > 0 {
		out = "Režija: "
		for _, p := range s.People {
			for _, t := range p.Types {
				if t == "DIRECTOR" {
					out += p.Name + " " + p.Surname + "\n"
				}
			}
		}
		out += "Igrajo: "
		for _, p := range s.People {
			for _, t := range p.Types {
				if t == "ACTOR" {
					out += p.Name + " " + p.Surname + ", "
				}
			}
		}
	}
	if len(out) == 0 {
		return ""
	}
	return out[:len(out)-2]
}

type Type struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type CountryT2 struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type Season struct {
	Number int `json:"number"`
}

type Episode struct {
	Number          int    `json:"number"`
	LongDescription string `json:"longDescription"`
	//DateAired       int    `json:"dateAired"`
}

type Language struct {
	LanguageId int    `json:"languageId"`
	Name       string `json:"name"`
}

type Genre struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type RatingT2 struct {
	ProviderId int     `json:"providerId"`
	Rating     float64 `json:"rating"`
}

type Person struct {
	Id      int      `json:"id"`
	Types   []string `json:"types"`
	Name    string   `json:"name"`
	Surname string   `json:"surname"`
}

func (s *Show) concatCountries() string {
	result := ""
	for i, country := range s.Countries {
		result += country.Name
		if i < len(s.Countries)-1 {
			result += ","
		}
	}
	return result
}

func (s *Show) concatGenres() string {
	result := ""
	for i, genre := range s.Genres {
		result += genre.Name
		if i < len(s.Countries)-1 {
			result += " | "
		}
	}
	return result
}

func runT2(outPath string) {

	client := &http.Client{}

	channels, err := getChannelList(client)
	if err != nil {
		fmt.Printf("Error getting chennel list: %s", err)
		return
	}

	tv := &Tv{
		GeneratorInfoName: "peter",
		GeneratorInfoUrl:  "peter",
	}

	channelsMap := make(map[int]string)
	channelIds := ""

	for _, channel := range channels {
		name := strings.ReplaceAll(channel.Name, "&", "and")
		name = strings.ReplaceAll(name, "(NEM)", "NEM")
		name = strings.ReplaceAll(name, "(ARD)", "ARD")
		for _, s := range channel.StreamResolutions {
			if s == "_4K" {
				name += " 4K"
			} else if s == "HD" {
				name += " HD"
			}
		}
		channelsMap[channel.Id] = name
		channelIds += strconv.Itoa(channel.Id) + ","
		tv.Channel = append(tv.Channel, &Channel{
			Id: name,
			DisplayName: &DisplayName{
				Lang:  "sl",
				Value: name,
			},
		})
	}

	channelIds = channelIds[:len(channelIds)-2]

	todayStart := time.Now().Truncate(time.Hour * 24)
	startTime := todayStart.UnixMilli()
	endTime := todayStart.Add(time.Hour * 72).UnixMilli()

	epg, err := getEpg(channelIds, startTime, endTime, client)
	if err != nil {
		fmt.Printf("Error getting epg: %s", err)
		return
	}

	var currentChannelId int

	for _, e := range epg {
		startInt, err := strconv.Atoi(e.StartTimestamp)
		if err != nil {
			fmt.Printf("Error converting start time for show %s value %s: %s", e.NameSingleLine, e.StartTimestamp, err)
			continue
		}
		endInt, err := strconv.Atoi(e.EndTimestamp)
		if err != nil {
			fmt.Printf("Error converting end time for show %s value %s: %s", e.NameSingleLine, e.EndTimestamp, err)
			continue
		}
		productionInt := 0
		if len(e.Show.ProductionFrom) > 0 {
			productionInt, err = strconv.Atoi(e.Show.ProductionFrom)
			if err != nil {
				fmt.Printf("Error converting production from for show %s value %s: %s", e.NameSingleLine, e.Show.ProductionFrom, err)
				continue
			}
		}
		start := time.UnixMilli(int64(startInt))
		end := time.UnixMilli(int64(endInt))

		//pn := channelsMap[e.ChannelId]
		//if strings.Contains(pn, "Sci Fi") {
		//	fmt.Println("in")
		//}
		if e.ChannelId != currentChannelId {
			fmt.Println(channelsMap[e.ChannelId])
			currentChannelId = e.ChannelId
		}
		p := &Programme{
			Channel: channelsMap[e.ChannelId],
			Start: parseTime(start.Year(), start.Month(), start.Day(), fmt.Sprintf("%02d:%02d"+
				"", start.Hour(), start.Minute())),
			Stop: parseTime(end.Year(), end.Month(), end.Day(), fmt.Sprintf("%02d:%02d", end.Hour(), end.Minute())),
			Description: &Description{
				Lang:  "sl",
				Value: e.Show.constructDescString(),
			},
			Title: &Title{
				Lang:  "sl",
				Value: e.NameSingleLine,
			},
			Country: &Country{
				Lang:  "sl",
				Value: e.Show.concatCountries(),
			},
			Category: &Category{
				Lang:  "sl",
				Value: e.Show.concatGenres(),
			},
		}
		if productionInt > 0 {
			p.Date = strconv.Itoa(time.UnixMilli(int64(productionInt)).Year())
		}
		if len(e.Show.Ratings) > 0 {
			p.Rating = &Rating{
				Value: fmt.Sprintf("%f", e.Show.Ratings[0].Rating),
			}
			if e.Show.Ratings[0].ProviderId == 1 {
				p.Rating = &Rating{
					Value:  fmt.Sprintf("%.1f", e.Show.Ratings[0].Rating),
					System: "IMDb",
				}
			}
		}
		//if len(e.Show.People) > 0 {
		//	p.Credits = &Credits{
		//		Actor:    nil,
		//		Director: nil,
		//	}
		//	for _, person := range e.Show.People {
		//		for _, t := range person.Types {
		//			if t == "ACTOR" {
		//				p.Credits.Actor = append(p.Credits.Actor, &Actor{
		//					Value: person.Name + " " + person.Surname,
		//				})
		//			}
		//			if t == "DIRECTOR" {
		//				p.Credits.Director = append(p.Credits.Director, &Director{
		//					Value: person.Name + " " + person.Surname,
		//				})
		//			}
		//		}
		//	}
		//}
		tv.Programme = append(tv.Programme, p)
	}

	out, err := xml.MarshalIndent(tv, " ", "  ")
	if err != nil {
		fmt.Println(err)
	}

	//out = []byte(strings.ReplaceAll(string(out), "&", "and"))

	err = ioutil.WriteFile(outPath+"epg_t2.xml", out, 0644)
	if err != nil {
		log.Fatalf("Error writing to ffile: %s", err)
	}
}

func makeT2Request(endpoint, data, method string, client *http.Client) ([]byte, error) {

	hashString := []byte(nonce + version + dataType + id + endpoint + data)

	url := fmt.Sprintf("%s/%s/%s/%s/%x/%s", baseUrl, version, dataType, id, md5.Sum(hashString), endpoint)

	req, err := http.NewRequest(method, url, strings.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return responseBody, err
}

func getEpg(channelIds string, starttime, endtime int64, client *http.Client) ([]Entry, error) {
	var result EpgResponse
	data := fmt.Sprintf("{\"locale\":\"sl-SI\",\"channelId\":[%s],\"startTime\":%d,\"endTime\":%d,\"includeBookmarks\":true,\"includeShow\":true}", channelIds, starttime, endtime)
	body, err := makeT2Request("client/tv/getEpg", data, "POST", client)
	if err != nil {
		return nil, err
	}
	//fmt.Println(string(body))
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}
	return result.Entries, nil
}

func getChannelList(client *http.Client) ([]T2Channel, error) {
	var result ChannelsResponse
	data := "{\"locale\":\"sl-SI\",\"type\":\"TV\"}"
	body, err := makeT2Request("client/channels/list", data, "POST", client)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}
	return result.Channels, nil
}

func concatStrings(input ...string) string {
	output := ""
	for i, in := range input {
		output += in
		if i < len(input)-1 {
			output += " | "
		}
	}
	return output
}
