package main

import "encoding/xml"

type Tv struct {
	XMLName           xml.Name     `xml:"tv"`
	GeneratorInfoName string       `xml:"generator-info-name,attr"`
	GeneratorInfoUrl  string       `xml:"generator-info-url,attr"`
	Channel           []*Channel   `xml:"channel"`
	Programme         []*Programme `xml:"programme"`
}

type Channel struct {
	XMLName     xml.Name     `xml:"channel"`
	Id          string       `xml:"id,attr"`
	DisplayName *DisplayName `xml:"display-name"`
	Icon        *Icon        `xml:"icon"`
	Url         string       `xml:"url,omitempty"`
}

type Programme struct {
	XMLName       xml.Name       `xml:"programme"`
	Channel       string         `xml:"channel,attr"`
	Clumpidx      string         `xml:"clumpidx,attr,omitempty"`
	PdcStart      string         `xml:"pdc-start,attr,omitempty"`
	ShowView      string         `xml:"showview,attr,omitempty"`
	Start         string         `xml:"start,attr,omitempty"`
	Stop          string         `xml:"stop,attr,omitempty"`
	VideoPlus     string         `xml:"videoplus,attr,omitempty"`
	VpsStart      string         `xml:"vps-start,attr,omitempty"`
	Title         *Title         `xml:"title"`
	Description   *Description   `xml:"desc,omitempty"`
	Category      *Category      `xml:"category,omitempty"`
	Date          string         `xml:"date,omitempty"`
	Icon          *Icon          `xml:"icon"`
	EpisodeNumber *EpisodeNumber `xml:"episode-num"`
	Country       *Country       `xml:"country"`
	Credits       *Credits       `xml:"credits"`
	Rating        *Rating        `xml:"rating"`
}

type DisplayName struct {
	XMLName xml.Name `xml:"display-name"`
	Lang    string   `xml:"lang,attr"`
	Value   string   `xml:",innerxml"`
}

type Icon struct {
	XMLName xml.Name `xml:"icon"`
	Src     string   `xml:"src,attr"`
	Height  string   `xml:"height,attr"`
	Width   string   `xml:"width,attr"`
}

type Title struct {
	XMLName xml.Name `xml:"title"`
	Lang    string   `xml:"lang,attr"`
	Value   string   `xml:",innerxml"`
}

type Description struct {
	XMLName xml.Name `xml:"desc"`
	Lang    string   `xml:"lang,attr"`
	Value   string   `xml:",innerxml"`
}

type Category struct {
	XMLName xml.Name `xml:"category"`
	Lang    string   `xml:"lang,attr"`
	Value   string   `xml:",innerxml"`
}

type EpisodeNumber struct {
	XMLName xml.Name `xml:"episode-num"`
	System  string   `xml:"system,attr"`
	Value   string   `xml:",innerxml"`
}

type Country struct {
	XMLName xml.Name `xml:"country"`
	Lang    string   `xml:"lang,attr"`
	Value   string   `xml:",innerxml"`
}

type Credits struct {
	XMLName  xml.Name `xml:"credits"`
	Actor    []*Actor `xml:"actor"`
	Director []*Director
}

type Actor struct {
	XMLName xml.Name `xml:"actor"`
	Value   string   `xml:",innerxml"`
}

type Director struct {
	XMLName xml.Name `xml:"director"`
	Value   string   `xml:",innerxml"`
}

type Rating struct {
	XMLName xml.Name `xml:"rating"`
	System  string   `xml:"system,attr"`
	Value   string   `xml:",innerxml"`
}
