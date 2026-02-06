package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"
)

var logger *slog.Logger

// initLogger sets up structured JSON logging
func initLogger(level slog.Level) {
	opts := &slog.HandlerOptions{
		Level: level,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger = slog.New(handler)
	slog.SetDefault(logger)
}

// logInfo logs an info message with optional attributes
func logInfo(msg string, attrs ...any) {
	if logger != nil {
		logger.Info(msg, attrs...)
	}
}

// logWarn logs a warning message with optional attributes
func logWarn(msg string, attrs ...any) {
	if logger != nil {
		logger.Warn(msg, attrs...)
	}
}

// logError logs an error message with optional attributes
func logError(msg string, attrs ...any) {
	if logger != nil {
		logger.Error(msg, attrs...)
	}
}

// logDebug logs a debug message with optional attributes
func logDebug(msg string, attrs ...any) {
	if logger != nil {
		logger.Debug(msg, attrs...)
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// requestContext holds request-scoped data for logging
type requestContext struct {
	VideoID  string
	CacheHit bool
}

type ctxKey string

const reqCtxKey ctxKey = "requestContext"

// setRequestContext stores request context for logging
func setRequestContext(r *http.Request, ctx *requestContext) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), reqCtxKey, ctx))
}

// getRequestContext retrieves request context for logging
func getRequestContext(r *http.Request) *requestContext {
	if ctx, ok := r.Context().Value(reqCtxKey).(*requestContext); ok {
		return ctx
	}
	return &requestContext{}
}

// loggingMiddleware logs HTTP requests with structured data
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Initialize request context
		r = setRequestContext(r, &requestContext{})

		// Wrap response writer to capture status
		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		// Process request
		next.ServeHTTP(wrapped, r)

		// Get request context for additional logging
		reqCtx := getRequestContext(r)

		// Build log attributes
		attrs := []any{
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", wrapped.status),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.String("ip", getClientIP(r)),
		}

		if reqCtx.VideoID != "" {
			attrs = append(attrs, slog.String("video_id", reqCtx.VideoID))
		}
		if r.Method == "POST" {
			attrs = append(attrs, slog.Bool("cache_hit", reqCtx.CacheHit))
		}

		// Log based on status code
		if wrapped.status >= 500 {
			logError("request failed", attrs...)
		} else if wrapped.status >= 400 {
			logWarn("request error", attrs...)
		} else {
			logInfo("request completed", attrs...)
		}
	})
}
