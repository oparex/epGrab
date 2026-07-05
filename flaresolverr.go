package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var fsHTTPClient = &http.Client{
	Timeout: 90 * time.Second,
}

type fsRequest struct {
	Cmd        string `json:"cmd"`
	URL        string `json:"url,omitempty"`
	MaxTimeout int    `json:"maxTimeout,omitempty"`
	Session    string `json:"session,omitempty"`
}

type fsSolution struct {
	Status   int    `json:"status"`
	Response string `json:"response"`
}

type fsResponse struct {
	Status   string     `json:"status"`
	Message  string     `json:"message"`
	Session  string     `json:"session,omitempty"`
	Solution fsSolution `json:"solution"`
}

func flaresolverrPost(endpoint string, req fsRequest) (fsResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return fsResponse{}, err
	}
	var lastErr error
	for attempt := 0; attempt < httpMaxAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(httpBaseBackoff << (attempt - 1))
		}
		resp, retryable, err := flaresolverrPostOnce(endpoint, body)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !retryable {
			return fsResponse{}, err
		}
	}
	return fsResponse{}, fmt.Errorf("after %d attempts: %w", httpMaxAttempts, lastErr)
}

func flaresolverrPostOnce(endpoint string, body []byte) (fsResponse, bool, error) {
	httpResp, err := fsHTTPClient.Post(endpoint+"/v1", "application/json", bytes.NewReader(body))
	if err != nil {
		return fsResponse{}, true, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		retryable := httpResp.StatusCode >= 500
		return fsResponse{}, retryable, fmt.Errorf("flaresolverr HTTP %d from %s/v1", httpResp.StatusCode, endpoint)
	}

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fsResponse{}, true, err
	}

	var res fsResponse
	if err := json.Unmarshal(raw, &res); err != nil {
		return fsResponse{}, false, fmt.Errorf("flaresolverr response parse: %w", err)
	}
	return res, false, nil
}

func flaresolverrGetHTML(targetURL, endpoint, sessionID string) (string, error) {
	resp, err := flaresolverrPost(endpoint, fsRequest{
		Cmd:        "request.get",
		URL:        targetURL,
		MaxTimeout: 60000,
		Session:    sessionID,
	})
	if err != nil {
		return "", fmt.Errorf("flaresolverr fetch %s: %w", targetURL, err)
	}
	if resp.Status != "ok" {
		return "", fmt.Errorf("flaresolverr error for %s: %s", targetURL, resp.Message)
	}
	if resp.Solution.Status != http.StatusOK {
		return "", fmt.Errorf("target returned HTTP %d for %s", resp.Solution.Status, targetURL)
	}
	return resp.Solution.Response, nil
}

func flaresolverrCreateSession(endpoint string) (string, error) {
	resp, err := flaresolverrPost(endpoint, fsRequest{Cmd: "sessions.create"})
	if err != nil {
		return "", fmt.Errorf("flaresolverr create session: %w", err)
	}
	if resp.Status != "ok" {
		return "", fmt.Errorf("flaresolverr create session failed: %s", resp.Message)
	}
	return resp.Session, nil
}

func flaresolverrDestroySession(endpoint, sessionID string) error {
	resp, err := flaresolverrPost(endpoint, fsRequest{Cmd: "sessions.destroy", Session: sessionID})
	if err != nil {
		return err
	}
	if resp.Status != "ok" {
		return fmt.Errorf("flaresolverr destroy session failed: %s", resp.Message)
	}
	return nil
}
