package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// extractVideoID pulls the video ID from various YouTube URL formats
func extractVideoID(url string) (string, error) {
	patterns := []string{
		`(?:youtube\.com/watch\?v=|youtu\.be/|youtube\.com/embed/|youtube\.com/v/)([a-zA-Z0-9_-]{11})`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(url)
		if len(matches) > 1 {
			return matches[1], nil
		}
	}

	// Check if it's already just a video ID
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]{11}$`, url); matched {
		return url, nil
	}

	return "", fmt.Errorf("could not extract video ID from: %s", url)
}

// fetchTranscript uses yt-dlp to download the transcript/subtitles
func fetchTranscript(url string) (string, error) {
	// Check if yt-dlp is installed
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return "", fmt.Errorf("yt-dlp is not installed. Install with: apt install yt-dlp (Linux) or brew install yt-dlp (Mac)")
	}

	// Fetch subtitles with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "yt-dlp",
		"--skip-download",
		"--write-auto-sub",
		"--write-sub",
		"--sub-lang", "en,en-US,en-GB",
		"--output", "/tmp/ytsummary-%(id)s",
		url,
	)

	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("yt-dlp timed out after 60 seconds")
	}
	if err != nil {
		return "", fmt.Errorf("yt-dlp failed: %s\n%s", err, string(output))
	}

	// Find and read the subtitle file
	videoID, _ := extractVideoID(url)
	subContent, err := findAndReadSubtitle(videoID)
	if err != nil {
		return "", fmt.Errorf("no subtitles available for this video: %w", err)
	}

	// Clean up the subtitle format to plain text
	return cleanSRT(subContent), nil
}

// findAndReadSubtitle looks for the downloaded subtitle file
func findAndReadSubtitle(videoID string) (string, error) {
	patterns := []string{
		fmt.Sprintf("/tmp/ytsummary-%s.en.vtt", videoID),
		fmt.Sprintf("/tmp/ytsummary-%s.en-US.vtt", videoID),
		fmt.Sprintf("/tmp/ytsummary-%s.en-GB.vtt", videoID),
	}

	for _, path := range patterns {
		content, err := os.ReadFile(path)
		if err == nil {
			// Clean up the temp file
			os.Remove(path)
			return string(content), nil
		}
	}

	return "", fmt.Errorf("subtitle file not found for video %s", videoID)
}

// cleanSubtitles removes timestamps and formatting from VTT/SRT content
func cleanSRT(content string) string {
	lines := strings.Split(content, "\n")
	var textLines []string
	var lastLine string

	// VTT format:
	// WEBVTT
	//
	// 00:00:00.000 --> 00:00:02.000
	// Text here
	//
	// SRT format is similar but with comma instead of dot

	timestampRe := regexp.MustCompile(`^\d{2}:\d{2}:\d{2}`)
	numberRe := regexp.MustCompile(`^\d+$`)
	tagRe := regexp.MustCompile(`<[^>]+>`)
	headerRe := regexp.MustCompile(`^(WEBVTT|Kind:|Language:)`)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines, numbers, timestamps, and VTT headers
		if line == "" || numberRe.MatchString(line) || timestampRe.MatchString(line) || headerRe.MatchString(line) {
			continue
		}

		// Remove HTML-like tags (common in auto-generated subs)
		line = tagRe.ReplaceAllString(line, "")
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		// Avoid duplicates (auto-subs often repeat lines)
		if line != lastLine {
			textLines = append(textLines, line)
			lastLine = line
		}
	}

	return strings.Join(textLines, " ")
}
