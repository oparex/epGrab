package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// parseTime formats a date plus an "HH:MM" clock time into the XMLTV timestamp
// format used throughout the EPG output, e.g. "20260624083000 +0200".
func parseTime(year int, month time.Month, day int, hour string) string {
	return fmt.Sprintf("%d%02d%02d%s00 +0200", year, month, day, strings.ReplaceAll(hour, ":", ""))
}

// userAgent is sent with every HTTP request so sites that do light
// user-agent based filtering treat us as an ordinary desktop browser.
const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"

// httpClient is a shared client reused across all plain-HTTP fetches. It keeps
// connections alive (important when fetching tens of thousands of pages) and
// caps how long any single request may take.
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 16,
		IdleConnTimeout:     90 * time.Second,
	},
}

// httpMaxAttempts and httpBaseBackoff control retries for transient failures.
// Over a full crawl (tens of thousands of requests) the local DNS resolver and
// the network occasionally hiccup — most visibly as "no such host" lookup
// failures — even though the host is perfectly reachable. Retrying with a short
// backoff turns those blips into successful fetches instead of lost channels.
const (
	httpMaxAttempts = 4
	httpBaseBackoff = 500 * time.Millisecond
)

// httpGetHTML fetches a URL with browser-like headers and returns the response
// body as a string, retrying transient network and server errors with an
// exponential backoff. It is used for sites that serve their content as plain
// server-rendered HTML (no JavaScript / Cloudflare challenge required), which
// is far cheaper than driving a headless browser.
func httpGetHTML(targetURL string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < httpMaxAttempts; attempt++ {
		if attempt > 0 {
			// 0.5s, 1s, 2s ... gives DNS / the network time to recover.
			time.Sleep(httpBaseBackoff << (attempt - 1))
		}

		body, retryable, err := httpGetOnce(targetURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retryable {
			return "", err
		}
	}
	return "", fmt.Errorf("after %d attempts: %w", httpMaxAttempts, lastErr)
}

// httpGetOnce performs a single fetch. The bool reports whether the failure is
// worth retrying: any transport-level error (DNS lookup, connection reset,
// timeout) and any 429/5xx response are retryable; other non-200 statuses
// (e.g. a real 404) are not.
func httpGetOnce(targetURL string) (string, bool, error) {
	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return "", false, err
	}

	// A realistic header set keeps us indistinguishable from a normal browser
	// for sites that do light user-agent based filtering.
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "sl-SI,sl;q=0.9,en-US;q=0.8,en;q=0.7")

	resp, err := httpClient.Do(req)
	if err != nil {
		// Transport errors (DNS, reset, timeout) are transient — retry.
		return "", true, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return "", retryable, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, targetURL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		// A read error mid-body is also transient.
		return "", true, err
	}
	return string(body), false, nil
}
