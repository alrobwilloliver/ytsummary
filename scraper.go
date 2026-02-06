package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// YouTubePlayerResponse - parsed from innertube API response
type YouTubePlayerResponse struct {
	VideoDetails struct {
		VideoID string `json:"videoId"`
		Title   string `json:"title"`
	} `json:"videoDetails"`
	Captions struct {
		PlayerCaptionsTracklistRenderer struct {
			CaptionTracks []CaptionTrack `json:"captionTracks"`
		} `json:"playerCaptionsTracklistRenderer"`
	} `json:"captions"`
	PlayabilityStatus struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
		LiveStreamability struct {
			LiveStreamabilityRenderer struct {
				VideoID string `json:"videoId"`
			} `json:"liveStreamabilityRenderer"`
		} `json:"liveStreamability"`
	} `json:"playabilityStatus"`
}

// CaptionTrack - single caption option
type CaptionTrack struct {
	BaseURL      string `json:"baseUrl"`
	LanguageCode string `json:"languageCode"`
	Kind         string `json:"kind"` // "asr" = auto-generated
}

// FetchResult - transcript with metadata
type FetchResult struct {
	VideoID    string
	Title      string
	Transcript string
	Language   string
}

// innertubeRequest is the request payload for YouTube's innertube API
type innertubeRequest struct {
	Context struct {
		Client struct {
			ClientName    string `json:"clientName"`
			ClientVersion string `json:"clientVersion"`
		} `json:"client"`
	} `json:"context"`
	VideoID string `json:"videoId"`
}

// HTTP client with timeout
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

// fetchPlayerResponse fetches video metadata using YouTube's innertube API
func fetchPlayerResponse(videoID string) (*YouTubePlayerResponse, error) {
	// Use Android client which reliably returns caption data
	reqBody := innertubeRequest{}
	reqBody.Context.Client.ClientName = "ANDROID"
	reqBody.Context.Client.ClientVersion = "19.09.37"
	reqBody.VideoID = videoID

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := "https://www.youtube.com/youtubei/v1/player?key=AIzaSyA8eiZmM1FaDVjRy-df2KTyQ_vz_yYM39w"
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "com.google.android.youtube/19.09.37 (Linux; U; Android 11) gzip")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch player response: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("rate limited by YouTube (429)")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("innertube API error: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var pr YouTubePlayerResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, fmt.Errorf("failed to parse player response: %w", err)
	}

	return &pr, nil
}

// checkPlayability checks if the video is playable and returns appropriate errors
func checkPlayability(pr *YouTubePlayerResponse) error {
	status := pr.PlayabilityStatus.Status
	reason := strings.ToLower(pr.PlayabilityStatus.Reason)

	switch status {
	case "UNPLAYABLE":
		return fmt.Errorf("Private video or unavailable")
	case "LOGIN_REQUIRED":
		if strings.Contains(reason, "age") {
			return fmt.Errorf("age-restricted video")
		}
		return fmt.Errorf("login required to view this video")
	case "ERROR":
		return fmt.Errorf("video error: %s", pr.PlayabilityStatus.Reason)
	}

	// Check for live stream
	if pr.PlayabilityStatus.LiveStreamability.LiveStreamabilityRenderer.VideoID != "" {
		return fmt.Errorf("live streams are not supported")
	}

	return nil
}

// selectCaptionTrack selects the best caption track for the given language
// Priority: exact match → prefix match → first available
func selectCaptionTrack(tracks []CaptionTrack, lang string) (*CaptionTrack, error) {
	if len(tracks) == 0 {
		return nil, fmt.Errorf("no subtitles available for this video")
	}

	// Exact match
	for i := range tracks {
		if tracks[i].LanguageCode == lang {
			return &tracks[i], nil
		}
	}

	// Prefix match (e.g., "en" matches "en-US", "en-GB")
	for i := range tracks {
		if strings.HasPrefix(tracks[i].LanguageCode, lang+"-") ||
			strings.HasPrefix(tracks[i].LanguageCode, lang) {
			return &tracks[i], nil
		}
	}

	// Also try matching if requested lang has prefix (e.g., "en-US" should match "en")
	langPrefix := strings.Split(lang, "-")[0]
	for i := range tracks {
		if tracks[i].LanguageCode == langPrefix {
			return &tracks[i], nil
		}
	}

	// Return first available track
	return &tracks[0], nil
}

// fetchCaptions fetches the caption content from the timedtext URL
func fetchCaptions(captionURL string) (string, error) {
	req, err := http.NewRequest("GET", captionURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create caption request: %w", err)
	}

	req.Header.Set("User-Agent", "com.google.android.youtube/19.09.37 (Linux; U; Android 11) gzip")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch captions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return "", fmt.Errorf("rate limited by YouTube (429)")
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch captions: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read caption response: %w", err)
	}

	if len(body) == 0 {
		return "", fmt.Errorf("empty caption response")
	}

	return string(body), nil
}

// parseTimedText parses YouTube's XML timedtext format into plain text
func parseTimedText(xmlContent string) string {
	// Extract text from <p>...</p> or <text>...</text> tags
	// Format: <p t="1360" d="1680">text here</p>
	// Or: <text start="1.36" dur="1.68">text here</text>

	var lines []string
	var lastLine string

	// Try <p> format first (format="3")
	pRegex := regexp.MustCompile(`<p[^>]*>([^<]*)</p>`)
	matches := pRegex.FindAllStringSubmatch(xmlContent, -1)

	if len(matches) == 0 {
		// Try <text> format
		textRegex := regexp.MustCompile(`<text[^>]*>([^<]*)</text>`)
		matches = textRegex.FindAllStringSubmatch(xmlContent, -1)
	}

	for _, match := range matches {
		if len(match) > 1 {
			text := match[1]
			// Decode HTML entities
			text = html.UnescapeString(text)
			text = strings.TrimSpace(text)

			// Skip empty lines and duplicates
			if text != "" && text != lastLine {
				lines = append(lines, text)
				lastLine = text
			}
		}
	}

	return strings.Join(lines, " ")
}

// fetchTranscriptDirect fetches transcript using YouTube's innertube API
func fetchTranscriptDirect(url, language string) (*FetchResult, error) {
	// Extract video ID
	videoID, err := extractVideoID(url)
	if err != nil {
		return nil, fmt.Errorf("invalid YouTube URL: %w", err)
	}

	// Fetch player response via innertube API
	pr, err := fetchPlayerResponse(videoID)
	if err != nil {
		return nil, err
	}

	// Check playability
	if err := checkPlayability(pr); err != nil {
		return nil, err
	}

	// Get caption tracks
	tracks := pr.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks
	if len(tracks) == 0 {
		return nil, fmt.Errorf("no subtitles available for this video")
	}

	// Select best caption track
	track, err := selectCaptionTrack(tracks, language)
	if err != nil {
		return nil, err
	}

	// Fetch captions
	captionContent, err := fetchCaptions(track.BaseURL)
	if err != nil {
		return nil, err
	}

	// Parse the timedtext XML to plain text
	var transcript string
	if strings.Contains(captionContent, "<timedtext") || strings.Contains(captionContent, "<transcript") {
		transcript = parseTimedText(captionContent)
	} else if strings.Contains(captionContent, "WEBVTT") {
		// Fallback to VTT parsing if we somehow get VTT format
		transcript = cleanSRT(captionContent)
	} else {
		// Try XML parsing anyway
		transcript = parseTimedText(captionContent)
	}

	if transcript == "" {
		return nil, fmt.Errorf("failed to parse caption content")
	}

	return &FetchResult{
		VideoID:    pr.VideoDetails.VideoID,
		Title:      pr.VideoDetails.Title,
		Transcript: transcript,
		Language:   track.LanguageCode,
	}, nil
}

// For backwards compatibility with tests that use extractPlayerResponse
// This function is deprecated in favor of fetchPlayerResponse
func extractPlayerResponse(html string) (*YouTubePlayerResponse, error) {
	// Find the start of ytInitialPlayerResponse
	marker := "ytInitialPlayerResponse = "
	startIdx := strings.Index(html, marker)
	if startIdx == -1 {
		marker = "var ytInitialPlayerResponse = "
		startIdx = strings.Index(html, marker)
		if startIdx == -1 {
			return nil, fmt.Errorf("ytInitialPlayerResponse not found in page")
		}
	}

	jsonStart := startIdx + len(marker)
	if jsonStart >= len(html) || html[jsonStart] != '{' {
		return nil, fmt.Errorf("expected JSON object after ytInitialPlayerResponse")
	}

	depth := 0
	inString := false
	escaped := false
	jsonEnd := jsonStart

	for i := jsonStart; i < len(html); i++ {
		ch := html[i]

		if escaped {
			escaped = false
			continue
		}

		if ch == '\\' && inString {
			escaped = true
			continue
		}

		if ch == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				jsonEnd = i + 1
				break
			}
		}
	}

	if depth != 0 {
		return nil, fmt.Errorf("unbalanced braces in ytInitialPlayerResponse")
	}

	jsonStr := html[jsonStart:jsonEnd]

	var pr YouTubePlayerResponse
	if err := json.Unmarshal([]byte(jsonStr), &pr); err != nil {
		return nil, fmt.Errorf("failed to parse ytInitialPlayerResponse: %w", err)
	}

	return &pr, nil
}
