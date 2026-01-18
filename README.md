# epGrab

A TV schedule scraper that aggregates EPG (Electronic Program Guide) data from multiple Slovenian TV providers into XMLTV format.

## Features

- Scrapes TV schedules from multiple sources
- Outputs standardized XMLTV format
- Supports both HTML scraping and JSON API sources

## Available Scrapers

| Scraper | Source | Type | Output File |
|---------|--------|------|-------------|
| `tvsporedi` | tvsporedi.si | HTML | `epg_tvsporedi.xml` |
| `t2` | T-2 (tv2go.t-2.net) | JSON API | `epg_t2.xml` |
| `a1` | A1 Slovenija (spored.a1.si) | JSON API | `epg_a1.xml` |
| `siol` | Siol (tv-spored.siol.net) | HTML | `epg_siol.xml` |

## Installation

### Prerequisites

- Go 1.17 or later

### Build

```bash
go build -o epGrab .
```

### Cross-compile for Linux

```bash
# AMD64
GOOS=linux GOARCH=amd64 go build -o epGrab-linux-amd64 .

# ARM
GOOS=linux GOARCH=arm go build -o epGrab-linux-arm .
```

## Usage

```bash
epGrab -scraper <name> [-out <path>]
```

### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-scraper` | Scraper to run (required) | - |
| `-out` | Output directory for XML files | `./` |

### Examples

Run the TvSporedi scraper:
```bash
./epGrab -scraper tvsporedi
```

Run the T-2 API scraper with custom output path:
```bash
./epGrab -scraper t2 -out /path/to/output/
```

Run all scrapers:
```bash
./epGrab -scraper all
```

## Output Format

All scrapers output XMLTV format files containing:

- Channel information (ID, name, icon)
- Programme listings with:
  - Title and description
  - Start and stop times
  - Category
  - Episode numbers
  - Credits (actors, directors)
  - Ratings

## Dependencies

- [Colly](https://github.com/gocolly/colly) - HTML scraping framework

## License

MIT
