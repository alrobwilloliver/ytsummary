package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

func init() {
	// Load .env file if present (silently ignore if missing)
	godotenv.Load()
}

var (
	// Config flags
	cacheDir   string
	llmModel   string
	llmAPIKey  string
	llmBaseURL string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "ytsummary",
		Short: "Summarize YouTube videos from their transcripts",
		Long: `A CLI tool that fetches YouTube video transcripts and generates summaries using an LLM.

Requires yt-dlp to be installed for transcript extraction.
Supports any OpenAI-compatible API for summarization.`,
	}

	// Summarize command
	summarizeCmd := &cobra.Command{
		Use:   "summarize <youtube-url>",
		Short: "Fetch transcript and summarize a YouTube video",
		Args:  cobra.ExactArgs(1),
		RunE:  runSummarize,
	}

	// Transcript command (just fetch, no summarize)
	transcriptCmd := &cobra.Command{
		Use:   "transcript <youtube-url>",
		Short: "Fetch and display the transcript only",
		Args:  cobra.ExactArgs(1),
		RunE:  runTranscript,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cacheDir, "cache-dir", "./transcripts", "Directory to cache transcripts")
	rootCmd.PersistentFlags().StringVar(&llmModel, "model", "", "LLM model to use (default: from YTSUMMARY_MODEL env)")
	rootCmd.PersistentFlags().StringVar(&llmAPIKey, "api-key", "", "LLM API key (default: from YTSUMMARY_API_KEY env)")
	rootCmd.PersistentFlags().StringVar(&llmBaseURL, "api-url", "", "LLM API base URL (default: from YTSUMMARY_API_URL env)")

	rootCmd.AddCommand(summarizeCmd)
	rootCmd.AddCommand(transcriptCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func log(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "→ "+format+"\n", args...)
}

func runSummarize(cmd *cobra.Command, args []string) error {
	url := args[0]

	log("Parsing URL...")
	videoID, err := extractVideoID(url)
	if err != nil {
		return fmt.Errorf("invalid YouTube URL: %w", err)
	}
	log("Video ID: %s", videoID)

	// Check cache first
	log("Checking cache...")
	transcript, err := getCachedTranscript(videoID)
	if err != nil {
		log("Not cached, fetching transcript via yt-dlp...")
		transcript, err = fetchTranscript(url)
		if err != nil {
			return fmt.Errorf("failed to fetch transcript: %w", err)
		}
		log("Transcript fetched (%d chars)", len(transcript))
		// Cache it
		if err := cacheTranscript(videoID, transcript); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to cache transcript: %v\n", err)
		} else {
			log("Cached transcript")
		}
	} else {
		log("Found cached transcript (%d chars)", len(transcript))
	}

	// Summarize
	log("Sending to LLM for summarization...")
	summary, err := summarize(transcript)
	if err != nil {
		return fmt.Errorf("failed to summarize: %w", err)
	}

	log("Done!\n")
	fmt.Println(summary)
	return nil
}

func runTranscript(cmd *cobra.Command, args []string) error {
	url := args[0]

	log("Parsing URL...")
	videoID, err := extractVideoID(url)
	if err != nil {
		return fmt.Errorf("invalid YouTube URL: %w", err)
	}
	log("Video ID: %s", videoID)

	// Check cache first
	log("Checking cache...")
	transcript, err := getCachedTranscript(videoID)
	if err != nil {
		log("Not cached, fetching transcript via yt-dlp...")
		transcript, err = fetchTranscript(url)
		if err != nil {
			return fmt.Errorf("failed to fetch transcript: %w", err)
		}
		log("Transcript fetched (%d chars)", len(transcript))
		// Cache it
		if err := cacheTranscript(videoID, transcript); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to cache transcript: %v\n", err)
		} else {
			log("Cached transcript")
		}
	} else {
		log("Found cached transcript (%d chars)", len(transcript))
	}

	log("Done!\n")
	fmt.Println(transcript)
	return nil
}
