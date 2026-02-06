package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestHealthEndpoint(t *testing.T) {
	// Setup
	tmpDir, _ := os.MkdirTemp("", "ytsummary-test-*")
	defer os.RemoveAll(tmpDir)
	cacheDir = tmpDir
	db = nil
	serverStartTime = time.Now()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("health endpoint returned %d, want %d", w.Code, http.StatusOK)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("status = %q, want %q", resp.Status, "ok")
	}

	if resp.UptimeSeconds < 0 {
		t.Errorf("uptime should be >= 0, got %d", resp.UptimeSeconds)
	}

	closeCache()
}

func TestHealthEndpointDegraded(t *testing.T) {
	// Setup
	tmpDir, _ := os.MkdirTemp("", "ytsummary-test-*")
	defer os.RemoveAll(tmpDir)
	cacheDir = tmpDir
	db = nil
	serverStartTime = time.Now()

	// Set last success to over an hour ago
	lastSuccessTime = time.Now().Add(-2 * time.Hour)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handleHealth(w, req)

	var resp HealthResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Status != "degraded" {
		t.Errorf("status = %q, want %q (last success > 1 hour ago)", resp.Status, "degraded")
	}

	// Reset for other tests
	lastSuccessTime = time.Time{}
	closeCache()
}

func TestTranscriptEndpointInvalidJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/transcript", bytes.NewBufferString("not json"))
	w := httptest.NewRecorder()

	handleTranscript(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}

	var resp ErrorResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Error != ErrInvalidRequest {
		t.Errorf("error = %q, want %q", resp.Error, ErrInvalidRequest)
	}
}

func TestTranscriptEndpointMissingURL(t *testing.T) {
	body := `{"language": "en"}`
	req := httptest.NewRequest("POST", "/transcript", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handleTranscript(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}

	var resp ErrorResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Error != ErrInvalidRequest {
		t.Errorf("error = %q, want %q", resp.Error, ErrInvalidRequest)
	}
}

func TestTranscriptEndpointInvalidURL(t *testing.T) {
	body := `{"url": "https://example.com/not-youtube"}`
	req := httptest.NewRequest("POST", "/transcript", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handleTranscript(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}

	var resp ErrorResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Error != ErrInvalidRequest {
		t.Errorf("error = %q, want %q", resp.Error, ErrInvalidRequest)
	}
}

func TestTranscriptEndpointCacheHit(t *testing.T) {
	// Setup cache with a transcript
	tmpDir, _ := os.MkdirTemp("", "ytsummary-test-*")
	defer os.RemoveAll(tmpDir)
	cacheDir = tmpDir
	db = nil

	videoID := "dQw4w9WgXcQ"
	lang := "en"
	transcript := "Test transcript content"

	cacheTranscript(videoID, lang, "Test Title", transcript)

	// Make request
	body := `{"url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "language": "en"}`
	req := httptest.NewRequest("POST", "/transcript", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handleTranscript(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}

	var resp TranscriptResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.VideoID != videoID {
		t.Errorf("video_id = %q, want %q", resp.VideoID, videoID)
	}
	if resp.Transcript != transcript {
		t.Errorf("transcript = %q, want %q", resp.Transcript, transcript)
	}
	if !resp.Cached {
		t.Error("cached should be true for cache hit")
	}
	if resp.Language != lang {
		t.Errorf("language = %q, want %q", resp.Language, lang)
	}

	closeCache()
}

func TestParseRequest(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantID    string
		wantLang  string
		wantError bool
	}{
		{
			name:     "valid request with language",
			body:     `{"url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "language": "es"}`,
			wantID:   "dQw4w9WgXcQ",
			wantLang: "es",
		},
		{
			name:     "valid request without language defaults to en",
			body:     `{"url": "https://youtu.be/dQw4w9WgXcQ"}`,
			wantID:   "dQw4w9WgXcQ",
			wantLang: "en",
		},
		{
			name:      "missing url",
			body:      `{"language": "en"}`,
			wantError: true,
		},
		{
			name:      "invalid json",
			body:      `not json`,
			wantError: true,
		},
		{
			name:      "invalid youtube url",
			body:      `{"url": "https://vimeo.com/123"}`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(tt.body))

			parsed, videoID, lang, err := parseRequest(req)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if parsed == nil {
				t.Fatal("parsed request is nil")
			}

			if videoID != tt.wantID {
				t.Errorf("videoID = %q, want %q", videoID, tt.wantID)
			}

			if lang != tt.wantLang {
				t.Errorf("lang = %q, want %q", lang, tt.wantLang)
			}
		})
	}
}

func TestAPIKeyAuth(t *testing.T) {
	// Create a simple handler that we'll wrap with auth
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}

	apiKey := "test-secret-key"

	// Create auth middleware manually for testing
	authHandler := func(w http.ResponseWriter, r *http.Request) {
		providedKey := r.Header.Get("X-API-Key")
		if providedKey == "" {
			// Try Authorization header
			auth := r.Header.Get("Authorization")
			if len(auth) > 7 && auth[:7] == "Bearer " {
				providedKey = auth[7:]
			}
		}
		if providedKey != apiKey {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid or missing API key")
			return
		}
		handler(w, r)
	}

	tests := []struct {
		name       string
		header     string
		headerVal  string
		wantStatus int
	}{
		{"no auth header", "", "", http.StatusUnauthorized},
		{"wrong api key", "X-API-Key", "wrong-key", http.StatusUnauthorized},
		{"correct X-API-Key", "X-API-Key", apiKey, http.StatusOK},
		{"correct Bearer token", "Authorization", "Bearer " + apiKey, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", nil)
			if tt.header != "" {
				req.Header.Set(tt.header, tt.headerVal)
			}
			w := httptest.NewRecorder()

			authHandler(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}
