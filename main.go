package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

var scrapers = map[string]func(string){
	"tvsporedi": runTvSporedi,
	"t2":        runT2,
	"siol":      runSiol,
	"a1":        runA1,
}

func main() {
	outPath := flag.String("out", "./", "Path to output xml")
	scraper := flag.String("scraper", "", "Scraper to run: tvsporedi, t2, siol, a1, or 'all' to run all")
	flag.Parse()

	if *scraper == "" {
		fmt.Println("Error: -scraper flag is required")
		fmt.Println("\nAvailable scrapers:")
		fmt.Println("  tvsporedi  - TvSporedi.si HTML scraper")
		fmt.Println("  t2         - T-2 JSON API scraper")
		fmt.Println("  siol       - Siol HTML scraper")
		fmt.Println("  a1         - A1 Slovenija JSON API scraper")
		fmt.Println("  all        - Run all scrapers")
		fmt.Println("\nUsage: epGrab -scraper <name> [-out <path>]")
		os.Exit(1)
	}

	if *scraper == "all" {
		for name, runFunc := range scrapers {
			fmt.Printf("Running %s scraper...\n", name)
			runFunc(*outPath)
		}
		return
	}

	scraperName := strings.ToLower(*scraper)
	runFunc, exists := scrapers[scraperName]
	if !exists {
		fmt.Printf("Error: unknown scraper '%s'\n", *scraper)
		fmt.Println("\nAvailable scrapers: tvsporedi, t2, siol, a1, all")
		os.Exit(1)
	}

	fmt.Printf("Running %s scraper...\n", scraperName)
	runFunc(*outPath)
}
