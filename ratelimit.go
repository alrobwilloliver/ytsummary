package main

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Rate limiting configuration (from Gap 12)
const (
	rateLimitPerMinute = 30            // requests per minute per IP
	rateLimitBurst     = 5             // burst allowance
	rateLimitCleanup   = 5 * time.Minute // cleanup stale entries
)

// ipRateLimiter tracks rate limiters per IP address
type ipRateLimiter struct {
	limiters map[string]*rateLimiterEntry
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var limiter *ipRateLimiter

func initRateLimiter() {
	limiter = &ipRateLimiter{
		limiters: make(map[string]*rateLimiterEntry),
		rate:     rate.Limit(float64(rateLimitPerMinute) / 60.0), // convert to per-second
		burst:    rateLimitBurst,
	}

	// Start cleanup goroutine
	go limiter.cleanup()
}

// getLimiter returns the rate limiter for a given IP, creating one if needed
func (l *ipRateLimiter) getLimiter(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, exists := l.limiters[ip]
	if !exists {
		entry = &rateLimiterEntry{
			limiter:  rate.NewLimiter(l.rate, l.burst),
			lastSeen: time.Now(),
		}
		l.limiters[ip] = entry
	} else {
		entry.lastSeen = time.Now()
	}

	return entry.limiter
}

// cleanup removes stale entries periodically
func (l *ipRateLimiter) cleanup() {
	ticker := time.NewTicker(rateLimitCleanup)
	for range ticker.C {
		l.mu.Lock()
		for ip, entry := range l.limiters {
			if time.Since(entry.lastSeen) > rateLimitCleanup {
				delete(l.limiters, ip)
			}
		}
		l.mu.Unlock()
	}
}

// allow checks if a request from the given IP is allowed
func (l *ipRateLimiter) allow(ip string) bool {
	return l.getLimiter(ip).Allow()
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (for reverse proxy setups)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP (original client)
		if idx := len(xff); idx > 0 {
			for i := 0; i < len(xff); i++ {
				if xff[i] == ',' {
					return xff[:i]
				}
			}
			return xff
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// rateLimitMiddleware wraps a handler with rate limiting
func rateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if limiter == nil {
			initRateLimiter()
		}

		ip := getClientIP(r)
		if !limiter.allow(ip) {
			w.Header().Set("Retry-After", "60")
			writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many requests, please try again later")
			return
		}

		next(w, r)
	}
}
