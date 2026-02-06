# ytsummary

A YouTube transcript fetcher and summarizer, designed to run reliably from a server.

## Goals

1. **Reliable server-side operation** - Fetch transcripts without browser automation
2. **Self-hosted** - Run on your own infrastructure (DigitalOcean droplet)
3. **Cost-effective** - Use cheap LLM models (Gemini Flash via OpenRouter)
4. **API-first** - HTTP API for integration with other services
5. **No external dependencies** - Pure Go, no yt-dlp or browser required

See [SPEC.md](SPEC.md) for the full technical specification and implementation plan.

## How It Works

This tool fetches YouTube transcripts using YouTube's internal **innertube API** - the same API that powers the official YouTube Android app.

```
┌─────────────┐     POST /youtubei/v1/player      ┌─────────────┐
│  ytsummary  │ ─────────────────────────────────▶│   YouTube   │
│             │    (Android client context)       │   Servers   │
│             │◀───────────────────────────────── │             │
└─────────────┘     Video metadata + caption URLs └─────────────┘
       │
       │ GET caption URL
       ▼
┌─────────────┐
│ XML Caption │ ──▶ Parse to plain text
│   Content   │
└─────────────┘
```

### Why This Approach?

The **official YouTube Data API v3** only allows downloading captions for videos you own. It cannot be used to get transcripts of arbitrary public videos.

All popular transcript tools (yt-dlp, youtube-transcript-api, etc.) use variations of the innertube approach because it's the only way to fetch public video transcripts without authentication.

### Limitations

| Scenario | Status |
|----------|--------|
| Public videos | ✅ Works |
| Age-restricted videos | ✅ Works (Android client bypasses age gates) |
| Private videos | ❌ Requires login (not supported) |
| Members-only content | ❌ Requires login (not supported) |
| Live streams | ❌ Blocked (no complete captions) |
| Videos without captions | ❌ Returns error |

### Risks

- **API Stability**: YouTube can change the innertube API at any time without notice
- **Rate Limiting**: Heavy use from a single IP may trigger 429 errors
- **Terms of Service**: Using internal APIs may violate YouTube's ToS (same gray area as yt-dlp)

## Installation

```bash
go install github.com/alrobwilloliver/ytsummary@latest
```

Or build from source:

```bash
git clone https://github.com/alrobwilloliver/ytsummary.git
cd ytsummary
go build -o ytsummary .
```

## Configuration

Set environment variables or use CLI flags:

| Variable | Flag | Description |
|----------|------|-------------|
| `YTSUMMARY_API_KEY` | `--api-key` | OpenRouter API key for summarization |
| `YTSUMMARY_MODEL` | `--model` | LLM model (default: `google/gemini-2.0-flash-001`) |
| `YTSUMMARY_API_URL` | `--api-url` | LLM API URL (default: OpenRouter) |
| `YTSUMMARY_SERVER_API_KEY` | `--server-api-key` | API key for HTTP server authentication |

## CLI Usage

### Fetch transcript only

```bash
ytsummary transcript https://youtu.be/dQw4w9WgXcQ
```

### Fetch and summarize

```bash
ytsummary summarize https://youtu.be/dQw4w9WgXcQ
```

### Specify language

```bash
ytsummary transcript --lang es https://youtu.be/dQw4w9WgXcQ
```

### Run as HTTP server

```bash
ytsummary serve --addr :8080 --server-api-key SECRET
```

## HTTP API

When running in server mode, the following endpoints are available:

### Health check

```bash
curl http://localhost:8080/health
```

### Fetch transcript

```bash
curl -X POST http://localhost:8080/transcript \
  -H "X-API-Key: SECRET" \
  -H "Content-Type: application/json" \
  -d '{"url": "https://youtu.be/dQw4w9WgXcQ", "language": "en"}'
```

### Fetch and summarize

```bash
curl -X POST http://localhost:8080/summarize \
  -H "X-API-Key: SECRET" \
  -H "Content-Type: application/json" \
  -d '{"url": "https://youtu.be/dQw4w9WgXcQ"}'
```

### Response Format

```json
{
  "video_id": "dQw4w9WgXcQ",
  "title": "Rick Astley - Never Gonna Give You Up",
  "transcript": "...",
  "summary": "...",
  "language": "en",
  "cached": false,
  "duration_ms": 1234
}
```

### Error Codes

| Code | Description |
|------|-------------|
| `no_captions` | Video has no captions available |
| `video_unavailable` | Video is private or doesn't exist |
| `age_restricted` | Video requires login (shouldn't happen with Android client) |
| `rate_limited` | Too many requests, try again later |
| `scrape_failed` | General fetch failure |

## Docker

```bash
# Build
docker build -t ytsummary .

# Run
docker run -p 8080:8080 \
  -e YTSUMMARY_API_KEY=your-openrouter-key \
  -e YTSUMMARY_SERVER_API_KEY=your-api-key \
  ytsummary
```

## Supported URL Formats

- `youtube.com/watch?v=VIDEO_ID`
- `youtu.be/VIDEO_ID`
- `youtube.com/shorts/VIDEO_ID`
- `youtube.com/live/VIDEO_ID`
- `youtube.com/embed/VIDEO_ID`
- `m.youtube.com/watch?v=VIDEO_ID`

## Requirements

- Go 1.22+

## Testing

```bash
# Unit tests
go test ./...

# Integration tests (makes real YouTube API calls)
go test -tags=integration -v
```

## License

MIT
