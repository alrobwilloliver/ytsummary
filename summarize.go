package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultModel = "google/gemini-2.0-flash-001"
const defaultAPIURL = "https://openrouter.ai/api/v1"
const maxChunkTokens = 100000 // Approximate, will chunk if transcript is very long

// summarize sends the transcript to an LLM and returns a summary
func summarize(transcript string) (string, error) {
	apiKey := getConfig(llmAPIKey, "YTSUMMARY_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("no API key provided. Set YTSUMMARY_API_KEY or use --api-key")
	}

	model := getConfig(llmModel, "YTSUMMARY_MODEL")
	if model == "" {
		model = defaultModel
	}

	apiURL := getConfig(llmBaseURL, "YTSUMMARY_API_URL")
	if apiURL == "" {
		apiURL = defaultAPIURL
	}

	// For very long transcripts, chunk and summarize each chunk
	chunks := chunkTranscript(transcript, maxChunkTokens)

	if len(chunks) == 1 {
		return summarizeChunk(chunks[0], apiKey, model, apiURL, false)
	}

	// Multi-chunk: summarize each, then combine
	var chunkSummaries []string
	for i, chunk := range chunks {
		fmt.Fprintf(os.Stderr, "Summarizing chunk %d/%d...\n", i+1, len(chunks))
		summary, err := summarizeChunk(chunk, apiKey, model, apiURL, true)
		if err != nil {
			return "", fmt.Errorf("failed to summarize chunk %d: %w", i+1, err)
		}
		chunkSummaries = append(chunkSummaries, summary)
	}

	// Combine chunk summaries into final summary
	combined := strings.Join(chunkSummaries, "\n\n---\n\n")
	return summarizeChunk(combined, apiKey, model, apiURL, false)
}

func summarizeChunk(text, apiKey, model, apiURL string, isPartial bool) (string, error) {
	prompt := `Summarize this YouTube video transcript. Provide:
1. A brief overview (2-3 sentences)
2. Key points (bullet list)
3. Any notable quotes or moments

Keep it concise but comprehensive.`

	if isPartial {
		prompt = `Summarize this section of a YouTube video transcript. Extract the key points and main ideas. Be thorough but concise.`
	}

	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": prompt},
			{"role": "user", "content": text},
		},
		"max_tokens": 2000,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", apiURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	return result.Choices[0].Message.Content, nil
}

// chunkTranscript splits text into chunks that fit within token limits
// This is a rough approximation - 1 token ≈ 4 characters
func chunkTranscript(text string, maxTokens int) []string {
	maxChars := maxTokens * 4

	if len(text) <= maxChars {
		return []string{text}
	}

	var chunks []string
	words := strings.Fields(text)
	var currentChunk strings.Builder

	for _, word := range words {
		if currentChunk.Len()+len(word)+1 > maxChars {
			chunks = append(chunks, currentChunk.String())
			currentChunk.Reset()
		}
		if currentChunk.Len() > 0 {
			currentChunk.WriteString(" ")
		}
		currentChunk.WriteString(word)
	}

	if currentChunk.Len() > 0 {
		chunks = append(chunks, currentChunk.String())
	}

	return chunks
}

// getConfig returns flag value if set, otherwise env var
func getConfig(flagVal, envKey string) string {
	if flagVal != "" {
		return flagVal
	}
	return os.Getenv(envKey)
}
