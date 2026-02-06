package main

import (
	"os"
	"strings"
	"testing"
)

func TestExtractPlayerResponse(t *testing.T) {
	tests := []struct {
		name        string
		fixturePath string
		wantVideoID string
		wantTitle   string
		wantErr     bool
	}{
		{
			name:        "normal video",
			fixturePath: "testdata/normal_video.html",
			wantVideoID: "dQw4w9WgXcQ",
			wantTitle:   "Rick Astley - Never Gonna Give You Up",
			wantErr:     false,
		},
		{
			name:        "video without captions",
			fixturePath: "testdata/no_captions.html",
			wantVideoID: "abc123def45",
			wantTitle:   "Video Without Captions",
			wantErr:     false,
		},
		{
			name:        "private video",
			fixturePath: "testdata/private_video.html",
			wantVideoID: "private12345",
			wantTitle:   "Private Video",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html, err := os.ReadFile(tt.fixturePath)
			if err != nil {
				t.Fatalf("failed to read fixture: %v", err)
			}

			pr, err := extractPlayerResponse(string(html))
			if (err != nil) != tt.wantErr {
				t.Errorf("extractPlayerResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if pr.VideoDetails.VideoID != tt.wantVideoID {
					t.Errorf("VideoID = %v, want %v", pr.VideoDetails.VideoID, tt.wantVideoID)
				}
				if pr.VideoDetails.Title != tt.wantTitle {
					t.Errorf("Title = %v, want %v", pr.VideoDetails.Title, tt.wantTitle)
				}
			}
		})
	}
}

func TestExtractPlayerResponse_NotFound(t *testing.T) {
	html := "<html><body>No player response here</body></html>"
	_, err := extractPlayerResponse(html)
	if err == nil {
		t.Error("expected error for missing ytInitialPlayerResponse")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestCheckPlayability_OK(t *testing.T) {
	html, _ := os.ReadFile("testdata/normal_video.html")
	pr, _ := extractPlayerResponse(string(html))

	err := checkPlayability(pr)
	if err != nil {
		t.Errorf("expected no error for OK status, got: %v", err)
	}
}

func TestCheckPlayability_Unplayable(t *testing.T) {
	html, _ := os.ReadFile("testdata/private_video.html")
	pr, _ := extractPlayerResponse(string(html))

	err := checkPlayability(pr)
	if err == nil {
		t.Error("expected error for UNPLAYABLE status")
	}
	if !strings.Contains(err.Error(), "Private video") {
		t.Errorf("expected 'Private video' in error, got: %v", err)
	}
}

func TestCheckPlayability_AgeRestricted(t *testing.T) {
	html, _ := os.ReadFile("testdata/age_restricted.html")
	pr, _ := extractPlayerResponse(string(html))

	err := checkPlayability(pr)
	if err == nil {
		t.Error("expected error for age-restricted video")
	}
	if !strings.Contains(err.Error(), "age-restricted") {
		t.Errorf("expected 'age-restricted' in error, got: %v", err)
	}
}

func TestCheckPlayability_LiveStream(t *testing.T) {
	html, _ := os.ReadFile("testdata/live_stream.html")
	pr, _ := extractPlayerResponse(string(html))

	err := checkPlayability(pr)
	if err == nil {
		t.Error("expected error for live stream")
	}
	if !strings.Contains(err.Error(), "live stream") {
		t.Errorf("expected 'live stream' in error, got: %v", err)
	}
}

func TestSelectCaptionTrack_ExactMatch(t *testing.T) {
	tracks := []CaptionTrack{
		{BaseURL: "url1", LanguageCode: "en", Kind: "asr"},
		{BaseURL: "url2", LanguageCode: "es", Kind: ""},
		{BaseURL: "url3", LanguageCode: "fr", Kind: ""},
	}

	track, err := selectCaptionTrack(tracks, "es")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if track.LanguageCode != "es" {
		t.Errorf("expected 'es', got %v", track.LanguageCode)
	}
}

func TestSelectCaptionTrack_PrefixMatch(t *testing.T) {
	tracks := []CaptionTrack{
		{BaseURL: "url1", LanguageCode: "en-US", Kind: ""},
		{BaseURL: "url2", LanguageCode: "en-GB", Kind: ""},
		{BaseURL: "url3", LanguageCode: "es", Kind: ""},
	}

	track, err := selectCaptionTrack(tracks, "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(track.LanguageCode, "en") {
		t.Errorf("expected 'en' prefix, got %v", track.LanguageCode)
	}
}

func TestSelectCaptionTrack_ReversePrefixMatch(t *testing.T) {
	tracks := []CaptionTrack{
		{BaseURL: "url1", LanguageCode: "en", Kind: "asr"},
		{BaseURL: "url2", LanguageCode: "es", Kind: ""},
	}

	// Request en-US but only "en" is available
	track, err := selectCaptionTrack(tracks, "en-US")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if track.LanguageCode != "en" {
		t.Errorf("expected 'en', got %v", track.LanguageCode)
	}
}

func TestSelectCaptionTrack_Fallback(t *testing.T) {
	tracks := []CaptionTrack{
		{BaseURL: "url1", LanguageCode: "ja", Kind: ""},
		{BaseURL: "url2", LanguageCode: "ko", Kind: ""},
	}

	// No English available, should return first track
	track, err := selectCaptionTrack(tracks, "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if track.LanguageCode != "ja" {
		t.Errorf("expected first track 'ja', got %v", track.LanguageCode)
	}
}

func TestSelectCaptionTrack_Empty(t *testing.T) {
	tracks := []CaptionTrack{}

	_, err := selectCaptionTrack(tracks, "en")
	if err == nil {
		t.Error("expected error for empty tracks")
	}
	if !strings.Contains(err.Error(), "no subtitles available") {
		t.Errorf("expected 'no subtitles available' in error, got: %v", err)
	}
}

func TestSelectCaptionTrack_FromFixture(t *testing.T) {
	html, _ := os.ReadFile("testdata/normal_video.html")
	pr, _ := extractPlayerResponse(string(html))

	tracks := pr.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks

	track, err := selectCaptionTrack(tracks, "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if track.LanguageCode != "en" {
		t.Errorf("expected 'en', got %v", track.LanguageCode)
	}
}

func TestSelectCaptionTrack_NoCaptions(t *testing.T) {
	html, _ := os.ReadFile("testdata/no_captions.html")
	pr, _ := extractPlayerResponse(string(html))

	tracks := pr.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks

	_, err := selectCaptionTrack(tracks, "en")
	if err == nil {
		t.Error("expected error for video without captions")
	}
	if !strings.Contains(err.Error(), "no subtitles available") {
		t.Errorf("expected 'no subtitles available' in error, got: %v", err)
	}
}

// TestCleanSRT_VTT tests the VTT cleaning functionality
func TestCleanSRT_VTT(t *testing.T) {
	vtt, err := os.ReadFile("testdata/sample.vtt")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	result := cleanSRT(string(vtt))

	// Should contain the lyrics
	if !strings.Contains(result, "Never gonna give you up") {
		t.Error("expected 'Never gonna give you up' in result")
	}
	if !strings.Contains(result, "Never gonna let you down") {
		t.Error("expected 'Never gonna let you down' in result")
	}

	// Should not contain timestamps
	if strings.Contains(result, "00:00") {
		t.Error("result should not contain timestamps")
	}

	// Should not contain WEBVTT header
	if strings.Contains(result, "WEBVTT") {
		t.Error("result should not contain WEBVTT header")
	}
}

// TestParseTimedText tests YouTube XML timedtext parsing
func TestParseTimedText(t *testing.T) {
	xml, err := os.ReadFile("testdata/sample_timedtext.xml")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	result := parseTimedText(string(xml))

	// Should contain the lyrics with HTML entities decoded
	if !strings.Contains(result, "We're no strangers to love") {
		t.Errorf("expected decoded HTML entities, got: %s", result)
	}
	if !strings.Contains(result, "Never gonna give you up") {
		t.Error("expected 'Never gonna give you up' in result")
	}

	// Should not contain XML tags
	if strings.Contains(result, "<p") || strings.Contains(result, "</p>") {
		t.Error("result should not contain XML tags")
	}

	// Should not contain timestamps
	if strings.Contains(result, "t=") || strings.Contains(result, "d=") {
		t.Error("result should not contain timestamp attributes")
	}
}

// TestErrorMapping verifies error messages match handleFetchError patterns
func TestErrorMapping(t *testing.T) {
	tests := []struct {
		name        string
		fixturePath string
		wantContain string
	}{
		{
			name:        "no subtitles error",
			fixturePath: "testdata/no_captions.html",
			wantContain: "no subtitles available",
		},
		{
			name:        "private video error",
			fixturePath: "testdata/private_video.html",
			wantContain: "Private video",
		},
		{
			name:        "age restricted error",
			fixturePath: "testdata/age_restricted.html",
			wantContain: "age-restricted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html, _ := os.ReadFile(tt.fixturePath)
			pr, _ := extractPlayerResponse(string(html))

			var err error

			// Check playability first
			err = checkPlayability(pr)
			if err == nil {
				// If playable, check captions
				tracks := pr.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks
				_, err = selectCaptionTrack(tracks, "en")
			}

			if err == nil {
				t.Fatalf("expected error for %s", tt.name)
			}

			if !strings.Contains(err.Error(), tt.wantContain) {
				t.Errorf("expected error to contain %q, got: %v", tt.wantContain, err)
			}
		})
	}
}
