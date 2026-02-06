package main

import (
	"os"
	"testing"
)

func TestCache(t *testing.T) {
	// Use temp directory for test cache
	tmpDir, err := os.MkdirTemp("", "ytsummary-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set cache directory
	cacheDir = tmpDir

	// Reset global db for clean test
	db = nil

	// Test caching a transcript
	videoID := "dQw4w9WgXcQ"
	lang := "en"
	title := "Never Gonna Give You Up"
	transcript := "We're no strangers to love..."

	err = cacheTranscript(videoID, lang, title, transcript)
	if err != nil {
		t.Fatalf("cacheTranscript() error = %v", err)
	}

	// Test retrieving it
	entry, err := getCachedTranscript(videoID, lang)
	if err != nil {
		t.Fatalf("getCachedTranscript() error = %v", err)
	}

	if entry.VideoID != videoID {
		t.Errorf("VideoID = %v, want %v", entry.VideoID, videoID)
	}
	if entry.Language != lang {
		t.Errorf("Language = %v, want %v", entry.Language, lang)
	}
	if entry.Title != title {
		t.Errorf("Title = %v, want %v", entry.Title, title)
	}
	if entry.Transcript != transcript {
		t.Errorf("Transcript = %v, want %v", entry.Transcript, transcript)
	}
	if entry.FetchedAt.IsZero() {
		t.Error("FetchedAt should not be zero")
	}

	// Test different language is separate cache entry
	langES := "es"
	transcriptES := "Nunca te voy a dejar..."

	err = cacheTranscript(videoID, langES, title, transcriptES)
	if err != nil {
		t.Fatalf("cacheTranscript(es) error = %v", err)
	}

	// Original English should still be there
	entryEN, err := getCachedTranscript(videoID, lang)
	if err != nil {
		t.Fatalf("getCachedTranscript(en) error = %v", err)
	}
	if entryEN.Transcript != transcript {
		t.Errorf("English transcript changed unexpectedly")
	}

	// Spanish should be different
	entryES, err := getCachedTranscript(videoID, langES)
	if err != nil {
		t.Fatalf("getCachedTranscript(es) error = %v", err)
	}
	if entryES.Transcript != transcriptES {
		t.Errorf("Spanish transcript = %v, want %v", entryES.Transcript, transcriptES)
	}

	// Test cache stats
	count, err := getCacheStats()
	if err != nil {
		t.Fatalf("getCacheStats() error = %v", err)
	}
	if count != 2 {
		t.Errorf("cache count = %v, want 2", count)
	}

	// Test cache miss
	_, err = getCachedTranscript("nonexistent", "en")
	if err == nil {
		t.Error("expected error for nonexistent video")
	}

	// Clean up
	closeCache()
}

func TestCacheUpdate(t *testing.T) {
	// Use temp directory for test cache
	tmpDir, err := os.MkdirTemp("", "ytsummary-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cacheDir = tmpDir
	db = nil

	videoID := "abc123xyz99"
	lang := "en"

	// Cache initial version
	err = cacheTranscript(videoID, lang, "Title v1", "Transcript v1")
	if err != nil {
		t.Fatalf("cacheTranscript() error = %v", err)
	}

	// Update with INSERT OR REPLACE
	err = cacheTranscript(videoID, lang, "Title v2", "Transcript v2")
	if err != nil {
		t.Fatalf("cacheTranscript() update error = %v", err)
	}

	// Should get updated version
	entry, err := getCachedTranscript(videoID, lang)
	if err != nil {
		t.Fatalf("getCachedTranscript() error = %v", err)
	}

	if entry.Title != "Title v2" {
		t.Errorf("Title = %v, want Title v2", entry.Title)
	}
	if entry.Transcript != "Transcript v2" {
		t.Errorf("Transcript = %v, want Transcript v2", entry.Transcript)
	}

	// Should still be only 1 entry
	count, err := getCacheStats()
	if err != nil {
		t.Fatalf("getCacheStats() error = %v", err)
	}
	if count != 1 {
		t.Errorf("cache count = %v, want 1", count)
	}

	closeCache()
}
