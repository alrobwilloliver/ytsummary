package main

import "testing"

func TestExtractVideoID(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    string
		wantErr bool
	}{
		// Standard formats
		{"standard watch", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ", false},
		{"watch without www", "https://youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ", false},
		{"short url", "https://youtu.be/dQw4w9WgXcQ", "dQw4w9WgXcQ", false},
		{"embed", "https://www.youtube.com/embed/dQw4w9WgXcQ", "dQw4w9WgXcQ", false},
		{"legacy v/", "https://www.youtube.com/v/dQw4w9WgXcQ", "dQw4w9WgXcQ", false},

		// New formats (Gap 17)
		{"shorts", "https://www.youtube.com/shorts/dQw4w9WgXcQ", "dQw4w9WgXcQ", false},
		{"live", "https://www.youtube.com/live/dQw4w9WgXcQ", "dQw4w9WgXcQ", false},
		{"mobile", "https://m.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ", false},

		// With extra params
		{"with timestamp", "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=120", "dQw4w9WgXcQ", false},
		{"with playlist", "https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf", "dQw4w9WgXcQ", false},
		{"short url with timestamp", "https://youtu.be/dQw4w9WgXcQ?t=30", "dQw4w9WgXcQ", false},

		// Raw video ID
		{"raw video id", "dQw4w9WgXcQ", "dQw4w9WgXcQ", false},

		// Invalid inputs
		{"empty string", "", "", true},
		{"random url", "https://example.com/video", "", true},
		{"too short id", "abc123", "", true},
		{"too long id", "dQw4w9WgXcQextra", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractVideoID(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractVideoID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("extractVideoID() = %v, want %v", got, tt.want)
			}
		})
	}
}
