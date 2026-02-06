package main

import (
	"fmt"
	"regexp"
	"strings"
)

// extractVideoID pulls the video ID from various YouTube URL formats
// Supported formats:
//   - youtube.com/watch?v=VIDEO_ID
//   - youtu.be/VIDEO_ID
//   - youtube.com/embed/VIDEO_ID
//   - youtube.com/v/VIDEO_ID
//   - youtube.com/shorts/VIDEO_ID
//   - youtube.com/live/VIDEO_ID
//   - m.youtube.com/watch?v=VIDEO_ID
//   - With extra params: ?v=VIDEO_ID&t=123
func extractVideoID(url string) (string, error) {
	patterns := []string{
		// Standard watch URL (including mobile)
		`(?:m\.)?youtube\.com/watch\?v=([a-zA-Z0-9_-]{11})`,
		// Short URL
		`youtu\.be/([a-zA-Z0-9_-]{11})`,
		// Embed and legacy URLs
		`youtube\.com/(?:embed|v)/([a-zA-Z0-9_-]{11})`,
		// Shorts
		`youtube\.com/shorts/([a-zA-Z0-9_-]{11})`,
		// Live streams
		`youtube\.com/live/([a-zA-Z0-9_-]{11})`,
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

// fetchTranscript fetches transcript using direct HTTP scraping
func fetchTranscript(url string) (string, error) {
	result, err := fetchTranscriptDirect(url, "en")
	if err != nil {
		return "", err
	}
	return result.Transcript, nil
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
