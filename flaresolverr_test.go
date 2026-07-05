package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlaresolverrGetHTML_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1", r.URL.Path)

		var req fsRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "request.get", req.Cmd)
		assert.Equal(t, "http://example.com", req.URL)
		assert.Equal(t, "sess1", req.Session)

		json.NewEncoder(w).Encode(fsResponse{
			Status:   "ok",
			Solution: fsSolution{Status: 200, Response: "<html>hello</html>"},
		})
	}))
	defer srv.Close()

	html, err := flaresolverrGetHTML("http://example.com", srv.URL, "sess1")
	require.NoError(t, err)
	assert.Equal(t, "<html>hello</html>", html)
}

func TestFlaresolverrGetHTML_noSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req fsRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "", req.Session, "empty sessionID must not send session field")
		json.NewEncoder(w).Encode(fsResponse{
			Status:   "ok",
			Solution: fsSolution{Status: 200, Response: "<html>ok</html>"},
		})
	}))
	defer srv.Close()

	_, err := flaresolverrGetHTML("http://example.com", srv.URL, "")
	require.NoError(t, err)
}

func TestFlaresolverrGetHTML_fsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(fsResponse{Status: "error", Message: "session not found"})
	}))
	defer srv.Close()

	_, err := flaresolverrGetHTML("http://example.com", srv.URL, "bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestFlaresolverrGetHTML_targetNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(fsResponse{
			Status:   "ok",
			Solution: fsSolution{Status: 403, Response: ""},
		})
	}))
	defer srv.Close()

	_, err := flaresolverrGetHTML("http://example.com", srv.URL, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

func TestFlaresolverrCreateSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req fsRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "sessions.create", req.Cmd)
		json.NewEncoder(w).Encode(fsResponse{Status: "ok", Session: "abc123"})
	}))
	defer srv.Close()

	sessionID, err := flaresolverrCreateSession(srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "abc123", sessionID)
}

func TestFlaresolverrDestroySession(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req fsRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "sessions.destroy", req.Cmd)
		assert.Equal(t, "abc123", req.Session)
		called = true
		json.NewEncoder(w).Encode(fsResponse{Status: "ok"})
	}))
	defer srv.Close()

	require.NoError(t, flaresolverrDestroySession(srv.URL, "abc123"))
	assert.True(t, called)
}
