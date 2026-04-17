package main

// This file ensures chromedp and goquery are kept as direct dependencies
// It can be removed once these libraries are actively imported elsewhere

import (
	_ "github.com/chromedp/chromedp"
	_ "github.com/PuerkitoBio/goquery"
)
