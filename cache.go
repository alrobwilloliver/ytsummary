package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// getCachedTranscript retrieves a transcript from the cache if it exists
func getCachedTranscript(videoID string) (string, error) {
	dir := cacheDir
	if dir == "" {
		dir = "./transcripts"
	}

	path := filepath.Join(dir, videoID+".txt")
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// cacheTranscript saves a transcript to the cache
func cacheTranscript(videoID, transcript string) error {
	dir := cacheDir
	if dir == "" {
		dir = "./transcripts"
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	path := filepath.Join(dir, videoID+".txt")
	return os.WriteFile(path, []byte(transcript), 0644)
}
