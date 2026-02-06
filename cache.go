package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// CacheEntry represents a cached transcript
type CacheEntry struct {
	VideoID    string
	Language   string
	Title      string
	Transcript string
	FetchedAt  time.Time
}

var db *sql.DB

// initCache initializes the SQLite database connection
func initCache() error {
	dbPath := cacheDir
	if dbPath == "" {
		dbPath = "./cache"
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(dbPath, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	dbFile := filepath.Join(dbPath, "transcripts.db")

	// Open with WAL mode and busy timeout for concurrent access
	dsn := fmt.Sprintf("file:%s?_busy_timeout=5000&_journal_mode=WAL&_synchronous=NORMAL", dbFile)
	var err error
	db, err = sql.Open("sqlite3", dsn)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// SQLite handles one writer at a time, limit connections
	db.SetMaxOpenConns(1)

	// Create table if not exists
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS transcripts (
			video_id TEXT NOT NULL,
			language TEXT NOT NULL,
			title TEXT,
			transcript TEXT NOT NULL,
			fetched_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (video_id, language)
		);
		CREATE INDEX IF NOT EXISTS idx_fetched_at ON transcripts(fetched_at);
	`)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	return nil
}

// closeCache closes the database connection
func closeCache() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

// getCachedTranscript retrieves a transcript from the cache if it exists
func getCachedTranscript(videoID, language string) (*CacheEntry, error) {
	if db == nil {
		if err := initCache(); err != nil {
			return nil, err
		}
	}

	var entry CacheEntry
	err := db.QueryRow(`
		SELECT video_id, language, title, transcript, fetched_at
		FROM transcripts
		WHERE video_id = ? AND language = ?
	`, videoID, language).Scan(
		&entry.VideoID,
		&entry.Language,
		&entry.Title,
		&entry.Transcript,
		&entry.FetchedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query cache: %w", err)
	}

	return &entry, nil
}

// cacheTranscript saves a transcript to the cache
func cacheTranscript(videoID, language, title, transcript string) error {
	if db == nil {
		if err := initCache(); err != nil {
			return err
		}
	}

	_, err := db.Exec(`
		INSERT OR REPLACE INTO transcripts (video_id, language, title, transcript, fetched_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, videoID, language, title, transcript)

	if err != nil {
		return fmt.Errorf("failed to cache transcript: %w", err)
	}

	return nil
}

// getCacheStats returns statistics about the cache
func getCacheStats() (int, error) {
	if db == nil {
		if err := initCache(); err != nil {
			return 0, err
		}
	}

	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM transcripts").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get cache stats: %w", err)
	}

	return count, nil
}
