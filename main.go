package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	outPath := flag.String("out", "./", "Path to output xml")
	scraper := flag.String("scraper", "", "Scraper to run: tvsporedi, t2, siol, a1, or 'all' to run all")
	flaresolverrURL := flag.String("flaresolverr-url", "http://localhost:8191", "FlareSolverr base URL (required for tvsporedi)")
	dryRun := flag.Bool("dry-run", false, "Print parsed data without writing XML (tvsporedi only)")
	flag.Parse()

	scrapers := map[string]func(string){
		"t2":   runT2,
		"siol": runSiol,
		"a1":   runA1,
		"tvsporedi": func(outPath string) {
			runTvSporedi(outPath, *flaresolverrURL, *dryRun)
		},
	}

	if *scraper == "" {
		fmt.Println("Error: -scraper flag is required")
		fmt.Println("\nAvailable scrapers:")
		fmt.Println("  tvsporedi  - TvSporedi.si HTML scraper (requires -flaresolverr-url)")
		fmt.Println("  t2         - T-2 JSON API scraper")
		fmt.Println("  siol       - Siol HTML scraper")
		fmt.Println("  a1         - A1 Slovenija JSON API scraper")
		fmt.Println("  all        - Run all scrapers")
		fmt.Println("\nUsage: epGrab -scraper <name> [-out <path>] [-flaresolverr-url <url>] [-dry-run]")
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
