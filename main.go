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
	cacheDir     string
	llmModel     string
	llmAPIKey    string
	llmBaseURL   string
	language     string
	serverAddr   string
	serverAPIKey string
)

const defaultLanguage = "en"

func main() {
	rootCmd := &cobra.Command{
		Use:   "ytsummary",
		Short: "Summarize YouTube videos from their transcripts",
		Long: `A CLI tool that fetches YouTube video transcripts and generates summaries using an LLM.

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

	// Serve command (HTTP API server)
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP API server",
		Long: `Start an HTTP server exposing the transcript and summarization API.

Endpoints:
  GET  /health     - Health check
  POST /transcript - Fetch transcript only
  POST /summarize  - Fetch transcript and summarize

Set YTSUMMARY_SERVER_API_KEY or use --server-api-key to require authentication.`,
		RunE: runServe,
	}
	serveCmd.Flags().StringVar(&serverAddr, "addr", ":8080", "Server listen address")
	serveCmd.Flags().StringVar(&serverAPIKey, "server-api-key", "", "API key for authentication (default: from YTSUMMARY_SERVER_API_KEY env)")

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cacheDir, "cache-dir", "./cache", "Directory for SQLite cache database")
	rootCmd.PersistentFlags().StringVar(&llmModel, "model", "", "LLM model to use (default: from YTSUMMARY_MODEL env)")
	rootCmd.PersistentFlags().StringVar(&llmAPIKey, "api-key", "", "LLM API key (default: from YTSUMMARY_API_KEY env)")
	rootCmd.PersistentFlags().StringVar(&llmBaseURL, "api-url", "", "LLM API base URL (default: from YTSUMMARY_API_URL env)")
	rootCmd.PersistentFlags().StringVar(&language, "lang", defaultLanguage, "Preferred transcript language (e.g., en, es, fr)")

	rootCmd.AddCommand(summarizeCmd)
	rootCmd.AddCommand(transcriptCmd)
	rootCmd.AddCommand(serveCmd)

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
	defer closeCache()

	log("Parsing URL...")
	videoID, err := extractVideoID(url)
	if err != nil {
		return fmt.Errorf("invalid YouTube URL: %w", err)
	}
	log("Video ID: %s", videoID)

	// Check cache first
	log("Checking cache for language '%s'...", language)
	var transcript string
	entry, err := getCachedTranscript(videoID, language)
	if err != nil {
		log("Not cached, fetching transcript...")
		transcript, err = fetchTranscript(url)
		if err != nil {
			return fmt.Errorf("failed to fetch transcript: %w", err)
		}
		log("Transcript fetched (%d chars)", len(transcript))
		// Cache it
		if err := cacheTranscript(videoID, language, "", transcript); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to cache transcript: %v\n", err)
		} else {
			log("Cached transcript")
		}
	} else {
		transcript = entry.Transcript
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
	defer closeCache()

	log("Parsing URL...")
	videoID, err := extractVideoID(url)
	if err != nil {
		return fmt.Errorf("invalid YouTube URL: %w", err)
	}
	log("Video ID: %s", videoID)

	// Check cache first
	log("Checking cache for language '%s'...", language)
	var transcript string
	entry, err := getCachedTranscript(videoID, language)
	if err != nil {
		log("Not cached, fetching transcript...")
		transcript, err = fetchTranscript(url)
		if err != nil {
			return fmt.Errorf("failed to fetch transcript: %w", err)
		}
		log("Transcript fetched (%d chars)", len(transcript))
		// Cache it
		if err := cacheTranscript(videoID, language, "", transcript); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to cache transcript: %v\n", err)
		} else {
			log("Cached transcript")
		}
	} else {
		transcript = entry.Transcript
		log("Found cached transcript (%d chars)", len(transcript))
	}

	log("Done!\n")
	fmt.Println(transcript)
	return nil
}

func runServe(cmd *cobra.Command, args []string) error {
	defer closeCache()

	// Get API key from flag or environment
	apiKey := serverAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("YTSUMMARY_SERVER_API_KEY")
	}

	return startServer(serverAddr, apiKey)
}
