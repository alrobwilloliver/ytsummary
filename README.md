# ytsummary

A YouTube transcript fetcher and summarizer, designed to run reliably from a server.

## Goals

1. **Reliable server-side operation** - Bypass YouTube's bot detection for datacenter IPs
2. **Self-hosted** - Run on your own infrastructure (DigitalOcean droplet)
3. **Cost-effective** - Use cheap LLM models (Gemini Flash via OpenRouter)
4. **API-first** - HTTP API for integration with other services

See [SPEC.md](SPEC.md) for the full technical specification and implementation plan.

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
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) (for transcript fetching, until Phase 1)

## License

MIT
