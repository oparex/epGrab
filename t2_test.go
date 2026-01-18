package main

import (
	"crypto/md5"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestT2RequestChannelEpg(t *testing.T) {
	a := "464830403846070"
	b := "6dace810-55d5-11e3-949a-0800200c9a66"
	xe := "json"
	version := "9.6"
	hn := "Catherine/api/" + version + "/" + xe + "/" + a + "/"
	nk := b + version + xe + a
	uri := "client/tv/getEpg"

	data := "{\"locale\":\"sl-SI\",\"channelId\":[1000260],\"startTime\":1653436800000,\"endTime\":1653523200000,\"includeBookmarks\":true,\"includeShow\":true}"

	hashString := []byte(nk + uri + data)
	fmt.Printf("%x", md5.Sum(hashString))

	req, err := http.NewRequest("POST", "https://tv2go.t-2.net/"+hn+fmt.Sprintf("%x", md5.Sum(hashString))+"/"+uri, strings.NewReader(data))
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Errored when sending request to the server")
		return
	}

	defer resp.Body.Close()
	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.Status)
	fmt.Println(string(responseBody))

}

func TestMD5(t *testing.T) {
	a := "464830403846070"
	b := "6dace810-55d5-11e3-949a-0800200c9a66"
	xe := "json"
	version := "9.6"
	nk := b + version + xe + a
	uri := "client/tv/getEpg"

	data := "{\"locale\":\"sl-SI\",\"channelId\":[1000260],\"startTime\":1653256800000,\"endTime\":1653516000000,\"imageInfo\":[{\"height\":500,\"width\":1100}],\"includeBookmarks\":true,\"includeShow\":true}"

	hashString := []byte(nk + uri + data)
	fmt.Printf("%x", md5.Sum(hashString))

	assert.Equal(t, "6d7bd75c059c9f2598862a08c549d93f", fmt.Sprintf("%x", md5.Sum(hashString)))

}

func TestGetChannelData(t *testing.T) {

	client := &http.Client{}

	res, err := getEpg("1000260,1000259", 1653256800000, 1653516000000, client)
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Println(res)
}

func TestGetChannelList(t *testing.T) {

	client := &http.Client{}

	channels, err := getChannelList(client)
	if err != nil {
		t.Error(err)
		return
	}

	fmt.Println(channels)
}

func TestMakeT2Request(t *testing.T) {

	client := &http.Client{}

	body, err := makeT2Request("client/channels/list", "", "GET", client)
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Println(body)

}

func TestRunT2(t *testing.T) {
	runT2("./")
}

func TestParseTime(t *testing.T) {
	start := time.Now()
	fmt.Println(parseTime(start.Year(), start.Month(), start.Day(), fmt.Sprintf("%d:%d", start.Hour(), start.Minute())))
}
