package main

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// TestHttpGetHTMLRetriesTransient verifies a transient server error is retried
// and eventually succeeds, the way an intermittent DNS/network blip would be.
func TestHttpGetHTMLRetriesTransient(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Fail the first two attempts with a retryable status, then succeed.
		if atomic.AddInt32(&hits, 1) <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("<html>ok</html>"))
	}))
	defer srv.Close()

	body, err := httpGetHTML(srv.URL)
	if err != nil {
		t.Fatalf("expected success after retries, got: %s", err)
	}
	if body != "<html>ok</html>" {
		t.Errorf("body = %q, want the success page", body)
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Errorf("server hit %d times, want 3 (2 failures + 1 success)", got)
	}
}

// TestHttpGetHTMLNoRetryOn404 verifies a genuine 404 is returned immediately
// without burning retry attempts.
func TestHttpGetHTMLNoRetryOn404(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if _, err := httpGetHTML(srv.URL); err == nil {
		t.Fatalf("expected error on 404")
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("server hit %d times, want 1 (404 is not retried)", got)
	}
}
