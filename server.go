package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// Server configuration (from Gap 11)
const (
	maxRequestBodySize     = 1024        // 1KB - only accepting JSON with URL + language
	serverReadTimeout      = 5 * time.Second
	serverWriteTimeout     = 120 * time.Second // Summarization can take time
	serverIdleTimeout      = 60 * time.Second
	gracefulShutdownTimeout = 30 * time.Second
)

// API request/response types (from Gap 1)

type TranscriptRequest struct {
	URL      string `json:"url"`
	Language string `json:"language,omitempty"` // defaults to "en"
}

type TranscriptResponse struct {
	VideoID    string `json:"video_id"`
	Title      string `json:"title,omitempty"`
	Transcript string `json:"transcript,omitempty"`
	Summary    string `json:"summary,omitempty"`
	Language   string `json:"language"`
	Cached     bool   `json:"cached"`
	DurationMS int64  `json:"duration_ms"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	VideoID string `json:"video_id,omitempty"`
}

type HealthResponse struct {
	Status               string `json:"status"` // "ok", "degraded", "unhealthy"
	CacheEntries         int    `json:"cache_entries"`
	UptimeSeconds        int64  `json:"uptime_seconds"`
	LastSuccess          string `json:"last_success,omitempty"`
	LastSuccessAgeSeconds int64  `json:"last_success_age_seconds,omitempty"`
}

// Error codes (from Gap 1)
const (
	ErrNoCaptions       = "no_captions"
	ErrVideoUnavailable = "video_unavailable"
	ErrAgeRestricted    = "age_restricted"
	ErrRateLimited      = "rate_limited"
	ErrScrapeFailed     = "scrape_failed"
	ErrLLMError         = "llm_error"
	ErrInvalidRequest   = "invalid_request"
)

var (
	serverStartTime time.Time
	lastSuccessTime time.Time
)

// startServer starts the HTTP server with graceful shutdown
func startServer(addr string, apiKey string) error {
	serverStartTime = time.Now()

	// Initialize logger (INFO level for production)
	initLogger(slog.LevelInfo)
	logInfo("starting server", slog.String("addr", addr))

	mux := http.NewServeMux()

	// Wrap handlers with API key auth if configured
	authMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if apiKey != "" {
				providedKey := r.Header.Get("X-API-Key")
				if providedKey == "" {
					providedKey = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
				}
				if providedKey != apiKey {
					writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid or missing API key")
					return
				}
			}
			next(w, r)
		}
	}

	// Initialize rate limiter
	initRateLimiter()

	// Routes (rate limiting applied to all endpoints except health)
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("POST /transcript", rateLimitMiddleware(authMiddleware(handleTranscript)))
	mux.HandleFunc("POST /summarize", rateLimitMiddleware(authMiddleware(handleSummarize)))

	// Create server with timeouts and logging
	server := &http.Server{
		Addr:         addr,
		Handler:      loggingMiddleware(http.MaxBytesHandler(mux, maxRequestBodySize)),
		ReadTimeout:  serverReadTimeout,
		WriteTimeout: serverWriteTimeout,
		IdleTimeout:  serverIdleTimeout,
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		logInfo("shutdown signal received, gracefully stopping server")

		ctx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			logError("server forced to shutdown", slog.String("error", err.Error()))
		}
	}()

	logInfo("server started", slog.String("addr", addr), slog.Bool("auth_enabled", apiKey != ""))

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		logError("server error", slog.String("error", err.Error()))
		return fmt.Errorf("server error: %w", err)
	}

	logInfo("server stopped")
	return nil
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	cacheCount, err := getCacheStats()
	status := "ok"
	if err != nil {
		status = "unhealthy"
		cacheCount = 0
	}

	resp := HealthResponse{
		Status:        status,
		CacheEntries:  cacheCount,
		UptimeSeconds: int64(time.Since(serverStartTime).Seconds()),
	}

	if !lastSuccessTime.IsZero() {
		resp.LastSuccess = lastSuccessTime.Format(time.RFC3339)
		resp.LastSuccessAgeSeconds = int64(time.Since(lastSuccessTime).Seconds())

		// Degraded if no success in over an hour
		if resp.LastSuccessAgeSeconds > 3600 && status == "ok" {
			resp.Status = "degraded"
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func handleTranscript(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	req, videoID, lang, err := parseRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrInvalidRequest, err.Error())
		return
	}

	// Update request context for logging
	reqCtx := getRequestContext(r)
	reqCtx.VideoID = videoID

	// Check cache
	cached := false
	var transcript, title string

	entry, err := getCachedTranscript(videoID, lang)
	if err == nil {
		cached = true
		transcript = entry.Transcript
		title = entry.Title
		logDebug("cache hit", slog.String("video_id", videoID), slog.String("language", lang))
	} else {
		logDebug("cache miss, fetching transcript", slog.String("video_id", videoID))
		// Fetch transcript
		transcript, err = fetchTranscript(req.URL)
		if err != nil {
			logWarn("fetch failed", slog.String("video_id", videoID), slog.String("error", err.Error()))
			handleFetchError(w, err, videoID)
			return
		}

		// Cache it
		_ = cacheTranscript(videoID, lang, "", transcript)
	}

	reqCtx.CacheHit = cached
	lastSuccessTime = time.Now()

	writeJSON(w, http.StatusOK, TranscriptResponse{
		VideoID:    videoID,
		Title:      title,
		Transcript: transcript,
		Language:   lang,
		Cached:     cached,
		DurationMS: time.Since(start).Milliseconds(),
	})
}

func handleSummarize(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	req, videoID, lang, err := parseRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrInvalidRequest, err.Error())
		return
	}

	// Update request context for logging
	reqCtx := getRequestContext(r)
	reqCtx.VideoID = videoID

	// Check cache for transcript
	cached := false
	var transcript, title string

	entry, err := getCachedTranscript(videoID, lang)
	if err == nil {
		cached = true
		transcript = entry.Transcript
		title = entry.Title
		logDebug("cache hit", slog.String("video_id", videoID), slog.String("language", lang))
	} else {
		logDebug("cache miss, fetching transcript", slog.String("video_id", videoID))
		// Fetch transcript
		transcript, err = fetchTranscript(req.URL)
		if err != nil {
			logWarn("fetch failed", slog.String("video_id", videoID), slog.String("error", err.Error()))
			handleFetchError(w, err, videoID)
			return
		}

		// Cache it
		_ = cacheTranscript(videoID, lang, "", transcript)
	}

	reqCtx.CacheHit = cached

	// Summarize
	logDebug("starting summarization", slog.String("video_id", videoID), slog.Int("transcript_len", len(transcript)))
	summary, err := summarize(transcript)
	if err != nil {
		logError("summarization failed", slog.String("video_id", videoID), slog.String("error", err.Error()))
		// Return transcript even if summarization fails (graceful degradation)
		writeJSON(w, http.StatusOK, TranscriptResponse{
			VideoID:    videoID,
			Title:      title,
			Transcript: transcript,
			Language:   lang,
			Cached:     cached,
			DurationMS: time.Since(start).Milliseconds(),
		})
		return
	}

	lastSuccessTime = time.Now()

	writeJSON(w, http.StatusOK, TranscriptResponse{
		VideoID:    videoID,
		Title:      title,
		Summary:    summary,
		Language:   lang,
		Cached:     cached,
		DurationMS: time.Since(start).Milliseconds(),
	})
}

func parseRequest(r *http.Request) (*TranscriptRequest, string, string, error) {
	var req TranscriptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, "", "", fmt.Errorf("invalid JSON: %w", err)
	}

	if req.URL == "" {
		return nil, "", "", fmt.Errorf("url is required")
	}

	videoID, err := extractVideoID(req.URL)
	if err != nil {
		return nil, "", "", fmt.Errorf("invalid YouTube URL: %w", err)
	}

	lang := req.Language
	if lang == "" {
		lang = defaultLanguage
	}

	return &req, videoID, lang, nil
}

func handleFetchError(w http.ResponseWriter, err error, videoID string) {
	errStr := err.Error()

	// Map common errors to error codes
	switch {
	case strings.Contains(errStr, "no subtitles available"):
		writeErrorWithVideo(w, http.StatusNotFound, ErrNoCaptions, "This video has no captions available", videoID)
	case strings.Contains(errStr, "Private video"):
		writeErrorWithVideo(w, http.StatusNotFound, ErrVideoUnavailable, "Video is private or unavailable", videoID)
	case strings.Contains(errStr, "age-restricted"):
		writeErrorWithVideo(w, http.StatusForbidden, ErrAgeRestricted, "Video is age-restricted", videoID)
	case strings.Contains(errStr, "429"), strings.Contains(errStr, "rate"):
		writeErrorWithVideo(w, http.StatusTooManyRequests, ErrRateLimited, "Rate limited by YouTube, try again later", videoID)
	default:
		writeErrorWithVideo(w, http.StatusBadGateway, ErrScrapeFailed, errStr, videoID)
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, ErrorResponse{
		Error:   code,
		Message: message,
	})
}

func writeErrorWithVideo(w http.ResponseWriter, status int, code, message, videoID string) {
	writeJSON(w, status, ErrorResponse{
		Error:   code,
		Message: message,
		VideoID: videoID,
	})
}
