package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimiter(t *testing.T) {
	// Reset limiter for clean test
	limiter = nil
	initRateLimiter()

	ip := "192.168.1.100"

	// Should allow burst of requests
	for i := 0; i < rateLimitBurst; i++ {
		if !limiter.allow(ip) {
			t.Errorf("request %d should be allowed (within burst)", i+1)
		}
	}

	// Next request should be rate limited (burst exhausted)
	if limiter.allow(ip) {
		t.Error("request after burst should be rate limited")
	}
}

func TestRateLimiterDifferentIPs(t *testing.T) {
	// Reset limiter for clean test
	limiter = nil
	initRateLimiter()

	ip1 := "192.168.1.1"
	ip2 := "192.168.1.2"

	// Exhaust burst for ip1
	for i := 0; i < rateLimitBurst; i++ {
		limiter.allow(ip1)
	}

	// ip1 should be rate limited
	if limiter.allow(ip1) {
		t.Error("ip1 should be rate limited")
	}

	// ip2 should still be allowed (different limiter)
	if !limiter.allow(ip2) {
		t.Error("ip2 should be allowed (separate limiter)")
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		wantIP     string
	}{
		{
			name:       "from RemoteAddr with port",
			remoteAddr: "192.168.1.1:12345",
			wantIP:     "192.168.1.1",
		},
		{
			name:       "from X-Forwarded-For single",
			remoteAddr: "10.0.0.1:12345",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.50"},
			wantIP:     "203.0.113.50",
		},
		{
			name:       "from X-Forwarded-For multiple",
			remoteAddr: "10.0.0.1:12345",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.50, 70.41.3.18, 150.172.238.178"},
			wantIP:     "203.0.113.50",
		},
		{
			name:       "from X-Real-IP",
			remoteAddr: "10.0.0.1:12345",
			headers:    map[string]string{"X-Real-IP": "203.0.113.99"},
			wantIP:     "203.0.113.99",
		},
		{
			name:       "X-Forwarded-For takes precedence over X-Real-IP",
			remoteAddr: "10.0.0.1:12345",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.50",
				"X-Real-IP":       "203.0.113.99",
			},
			wantIP: "203.0.113.50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			got := getClientIP(req)
			if got != tt.wantIP {
				t.Errorf("getClientIP() = %q, want %q", got, tt.wantIP)
			}
		})
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	// Reset limiter for clean test
	limiter = nil

	// Create a simple handler that returns 200
	handler := rateLimitMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Make requests up to burst limit - should all succeed
	for i := 0; i < rateLimitBurst; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.50:12345"
		w := httptest.NewRecorder()

		handler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("request %d: got status %d, want %d", i+1, w.Code, http.StatusOK)
		}
	}

	// Next request should be rate limited
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.50:12345"
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("rate limited request: got status %d, want %d", w.Code, http.StatusTooManyRequests)
	}

	// Check Retry-After header
	if w.Header().Get("Retry-After") == "" {
		t.Error("rate limited response should include Retry-After header")
	}
}
