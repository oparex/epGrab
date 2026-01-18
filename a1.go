package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func runA1(outPath string) {
	startYear, startMonth, startDay := time.Now().Date()
	endYear, endMonth, endDay := time.Now().AddDate(0, 0, 3).Date()
	url := fmt.Sprintf("https://spored.a1.si/api/epg/channels?startDate=%d-%d-%d&endDate=%d-%d-%d", startYear, startMonth, startDay, endYear, endMonth, endDay)
	response, err := http.Get(url)
	if err != nil {
		log.Fatalf("Failed to send request %s: %v", url, err)
	}
	defer response.Body.Close()

	// Check response status
	if response.StatusCode != http.StatusOK {
		log.Fatalf("Unexpected status for %s: %s", url, response.Status)
	}

	var channels []Program
	if err := json.NewDecoder(response.Body).Decode(&channels); err != nil {
		log.Fatalf("Failed to decode JSON: %v", err)
	}

	tv := &Tv{
		GeneratorInfoName: "peter",
		GeneratorInfoUrl:  "peter",
	}

	for _, channel := range channels {

		tv.Channel = append(tv.Channel, &Channel{
			Id: channel.Name,
			DisplayName: &DisplayName{
				Lang:  "sl",
				Value: channel.Name,
			},
		})

		fmt.Println(channel.Name)

		for _, schedule := range channel.Schedules {
			dateComponents := strings.Split(schedule.Time, "-")
			year, _ := strconv.Atoi(dateComponents[0])
			month, _ := strconv.Atoi(dateComponents[1])
			day, _ := strconv.Atoi(dateComponents[2])

			var previousProgramme *Programme

			for _, program := range schedule.Programs {
				targetUrl := fmt.Sprintf("https://spored.a1.si/api/epg/program/%d", program.Id)
				responseShow, err := http.Get(targetUrl)
				if err != nil {
					log.Fatalf("Failed to send request to %s: %v", targetUrl, err)
				}

				// Check response status
				if responseShow.StatusCode != http.StatusOK {
					log.Fatalf("Unexpected status %s: %s", targetUrl, responseShow.Status)
				}

				//body, err := io.ReadAll(responseShow.Body)
				//if err != nil {
				//	log.Fatalf("Failed to read body: %v", err)
				//}
				//
				var show ShowA1
				//err = json.Unmarshal(body, &show)
				//if err != nil {
				//	log.Fatalf("Failed to unmarshal JSON: %v", err)
				//}
				if err := json.NewDecoder(responseShow.Body).Decode(&show); err != nil {
					log.Fatalf("Failed to decode JSON: %v", err)
				}

				description := parseDescription(show.Description)

				programme := &Programme{
					Channel: channel.Name,
					Start:   parseTime(year, time.Month(month), day, program.StartTime),
					Stop:    "",
					Title: &Title{
						Lang:  "sl",
						Value: program.Title,
					},
					Description: &Description{
						Lang:  "sl",
						Value: description,
					},
					Category:      getCategory(show.Categories),
					Date:          schedule.Time,
					Icon:          nil,
					EpisodeNumber: nil,
					Country:       nil,
					Credits:       parseCredits(show.Cast, show.Directors),
					Rating: &Rating{
						System: "IMDB",
						Value:  fmt.Sprintf("%f", show.ImdbRating),
					},
				}

				tv.Programme = append(tv.Programme, programme)
				responseShow.Body.Close()

				if previousProgramme != nil {
					previousProgramme.Stop = programme.Start
				}
				previousProgramme = programme
			}
		}
	}

	out, err := xml.MarshalIndent(tv, " ", "  ")
	if err != nil {
		fmt.Println(err)
	}

	//out = []byte(strings.ReplaceAll(string(out), "&", "and"))

	err = os.WriteFile(outPath+"epg_a1.xml", out, 0644)
	if err != nil {
		log.Fatalf("Error writing to ffile: %s", err)
	}

}

func getCategory(categories []string) *Category {
	if len(categories) == 0 {
		return nil
	}
	return &Category{
		Lang:  "sl",
		Value: categories[0],
	}
}

func parseDescription(rawData string) string {
	returnString := strings.ReplaceAll(rawData, "Izvirni naslov: ", "")
	return returnString[strings.Index(returnString, "\n")+1:]
}

func parseCredits(cast []interface{}, directors []string) *Credits {
	var credits Credits
	for _, c := range cast {
		if len(c.(string)) == 0 {
			continue
		}
		credits.Actor = append(credits.Actor, &Actor{Value: c.(string)})
	}
	for _, d := range directors {
		if len(d) == 0 {
			continue
		}
		credits.Director = append(credits.Director, &Director{Value: d})
	}
	return &credits
}

type Program struct {
	Id            string     `json:"id"`
	Name          string     `json:"name"`
	ThumbnailUrl  string     `json:"thumbnailUrl"`
	ChannelNumber int        `json:"channelNumber"`
	Packages      []string   `json:"packages"`
	Schedules     []Schedule `json:"schedules"`
}

type Schedule struct {
	Time     string      `json:"schedule"`
	Programs []ProgramA1 `json:"programs"`
}

type ProgramA1 struct {
	Id         int      `json:"id"`
	Title      string   `json:"title"`
	StartTime  string   `json:"startTime"`
	Categories []string `json:"categories"`
}

type ShowA1 struct {
	OriginalTitle string        `json:"originalTitle"`
	ImdbRating    float64       `json:"imdbRating"`
	Rating        float64       `json:"rating"`
	Description   string        `json:"description"`
	ImageUris     []string      `json:"imageUris"`
	Cast          []interface{} `json:"cast"`
	Genres        []string      `json:"genres"`
	Directors     []string      `json:"directors"`
	Id            int           `json:"id"`
	Title         string        `json:"title"`
	StartTime     string        `json:"startTime"`
	Categories    []string      `json:"categories"`
}

func extractKeys(jsn []byte, keys map[string]struct{}) error {
	var result map[string]interface{}
	err := json.Unmarshal(jsn, &result)
	if err != nil {
		return err
	}

	for key := range result {
		keys[key] = struct{}{}
	}

	return nil
}
