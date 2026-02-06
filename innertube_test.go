//go:build integration

package main

import (
	"strings"
	"testing"
	"time"
)

// Integration tests for innertube API - run with: go test -tags=integration -v

// Test videos for different scenarios
var testVideos = map[string]struct {
	url         string
	expectError string // empty = expect success
	description string
}{
	"public_with_captions": {
		url:         "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		expectError: "",
		description: "Rick Astley - public video with captions",
	},
	"public_no_captions": {
		url:         "https://www.youtube.com/watch?v=ZGnVUamCMfU",
		expectError: "no subtitles",
		description: "Video that might not have captions",
	},
	"private_video": {
		url:         "https://www.youtube.com/watch?v=private12345",
		expectError: "unavailable",
		description: "Non-existent/private video ID",
	},
	"unlisted": {
		// Unlisted videos should work if you have the URL
		url:         "https://www.youtube.com/watch?v=jNQXAC9IVRw", // "Me at the zoo" - first YouTube video
		expectError: "",
		description: "First YouTube video (public)",
	},
	"short_video": {
		url:         "https://www.youtube.com/shorts/dQw4w9WgXcQ",
		expectError: "",
		description: "Same video via shorts URL format",
	},
	"different_language": {
		url:         "https://www.youtube.com/watch?v=kJQP7kiw5Fk", // Despacito - Spanish
		expectError: "",
		description: "Spanish video with Spanish captions",
	},
}

func TestInnertubePublicVideo(t *testing.T) {
	result, err := fetchTranscriptDirect("https://www.youtube.com/watch?v=dQw4w9WgXcQ", "en")
	if err != nil {
		t.Fatalf("failed to fetch public video: %v", err)
	}

	if result.VideoID != "dQw4w9WgXcQ" {
		t.Errorf("wrong video ID: got %s, want dQw4w9WgXcQ", result.VideoID)
	}

	if result.Title == "" {
		t.Error("expected non-empty title")
	}

	if len(result.Transcript) < 100 {
		t.Errorf("transcript too short: %d chars", len(result.Transcript))
	}

	if !strings.Contains(strings.ToLower(result.Transcript), "never gonna give you up") {
		t.Error("expected transcript to contain 'never gonna give you up'")
	}

	t.Logf("Title: %s", result.Title)
	t.Logf("Language: %s", result.Language)
	t.Logf("Transcript length: %d chars", len(result.Transcript))
}

func TestInnertubePrivateVideo(t *testing.T) {
	_, err := fetchTranscriptDirect("https://www.youtube.com/watch?v=private12345", "en")
	if err == nil {
		t.Fatal("expected error for non-existent video")
	}

	errStr := strings.ToLower(err.Error())
	// Should get some kind of error about video not being available
	if !strings.Contains(errStr, "unavailable") &&
		!strings.Contains(errStr, "private") &&
		!strings.Contains(errStr, "error") {
		t.Errorf("unexpected error message: %v", err)
	}
	t.Logf("Got expected error: %v", err)
}

func TestInnertubeRateLimiting(t *testing.T) {
	// Make several rapid requests to test rate limiting behavior
	const numRequests = 5
	var successCount, errorCount int
	var lastError error

	t.Logf("Making %d rapid requests to test rate limiting...", numRequests)

	for i := 0; i < numRequests; i++ {
		_, err := fetchTranscriptDirect("https://www.youtube.com/watch?v=dQw4w9WgXcQ", "en")
		if err != nil {
			errorCount++
			lastError = err
			if strings.Contains(err.Error(), "429") {
				t.Logf("Request %d: Rate limited (429)", i+1)
			} else {
				t.Logf("Request %d: Error - %v", i+1, err)
			}
		} else {
			successCount++
			t.Logf("Request %d: Success", i+1)
		}
		// Small delay between requests
		time.Sleep(100 * time.Millisecond)
	}

	t.Logf("Results: %d success, %d errors", successCount, errorCount)

	if errorCount > 0 && lastError != nil {
		if strings.Contains(lastError.Error(), "429") {
			t.Log("WARNING: Rate limiting detected with just a few requests")
		}
	}

	if successCount == 0 {
		t.Fatal("All requests failed")
	}
}

func TestInnertubeLanguageSelection(t *testing.T) {
	// Test Spanish video with Spanish language preference
	result, err := fetchTranscriptDirect("https://www.youtube.com/watch?v=kJQP7kiw5Fk", "es")
	if err != nil {
		// Might not have Spanish captions, try English
		result, err = fetchTranscriptDirect("https://www.youtube.com/watch?v=kJQP7kiw5Fk", "en")
		if err != nil {
			t.Skipf("Could not fetch captions: %v", err)
		}
	}

	t.Logf("Title: %s", result.Title)
	t.Logf("Language: %s", result.Language)
	t.Logf("Transcript length: %d chars", len(result.Transcript))
}

func TestInnertubeErrorMessages(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectError string
	}{
		{
			name:        "invalid_video_id",
			url:         "https://www.youtube.com/watch?v=INVALID123",
			expectError: "", // Will fail with some error
		},
		{
			name:        "malformed_url",
			url:         "not-a-valid-url",
			expectError: "could not extract video ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := fetchTranscriptDirect(tt.url, "en")
			if err == nil {
				t.Log("Unexpectedly succeeded")
				return
			}

			t.Logf("Error: %v", err)

			if tt.expectError != "" && !strings.Contains(err.Error(), tt.expectError) {
				t.Errorf("expected error containing %q, got: %v", tt.expectError, err)
			}
		})
	}
}

func TestInnertubePlayerResponse(t *testing.T) {
	// Test the raw player response to understand what data we get
	pr, err := fetchPlayerResponse("dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("failed to fetch player response: %v", err)
	}

	t.Logf("Video ID: %s", pr.VideoDetails.VideoID)
	t.Logf("Title: %s", pr.VideoDetails.Title)
	t.Logf("Playability Status: %s", pr.PlayabilityStatus.Status)
	t.Logf("Caption tracks: %d", len(pr.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks))

	for i, track := range pr.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks {
		t.Logf("  Track %d: %s (kind: %s)", i, track.LanguageCode, track.Kind)
	}
}
