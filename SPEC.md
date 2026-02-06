# YouTube Transcript Summarizer - Technical Specification

**Status:** Living document - Last updated: 2026-02-05
**Author:** Alan Oliver
**Goal:** Reliable, cheap, self-hosted YouTube transcript extraction + summarization

---

## Overview

A system to fetch YouTube video transcripts and summarize them using an LLM, designed to:
1. Work reliably from a server (bypass YouTube bot detection)
2. Integrate with Atlas (WhatsApp bot) as a skill
3. Be cost-effective for personal use
4. Showcase DevOps/infrastructure skills on GitHub

---

## The Problem

YouTube aggressively blocks datacenter IPs from accessing video data. Even with valid cookies, server requests get flagged as bots. This is because:

1. **IP Reputation:** Datacenter IP ranges are known and blocked
2. **Browser Fingerprinting:** Missing browser-like headers/behavior
3. **Rate Limiting:** Too many requests from same IP
4. **PO Tokens:** YouTube now requires "Proof of Origin" tokens

**What works locally but fails on servers:**
- yt-dlp with cookies
- Direct API requests
- Simple HTTP scraping

---

## Solution Architecture

### Approach: Headless Browser + Proxy Rotation

```
┌─────────────────────────────────────────────────────────────┐
│                    DigitalOcean                              │
│  ┌─────────────────┐    ┌─────────────────────────────────┐ │
│  │  Scraper Service │───▶│  Proxy Pool (Squid Droplets)   │ │
│  │  (Go + Playwright)│    │  proxy-1  proxy-2  proxy-3    │ │
│  └────────┬─────────┘    └─────────────────────────────────┘ │
│           │                          │                       │
│           ▼                          ▼                       │
│  ┌─────────────────┐         ┌──────────────┐               │
│  │  Atlas/Clawdbot │         │   YouTube    │               │
│  │  (calls scraper)│         │   (target)   │               │
│  └─────────────────┘         └──────────────┘               │
└─────────────────────────────────────────────────────────────┘
```

### Components

#### 1. Transcript Scraper Service
- **Language:** Go (consistent with existing ytsummary)
- **Method:** Headless browser (Playwright/Rod) OR direct API exploitation
- **Hosting:** DigitalOcean droplet ($6/month)
- **API:** Simple HTTP endpoint for Atlas to call

#### 2. Proxy Pool
- **Option A:** Self-hosted Squid proxies on cheap droplets ($4/month each)
- **Option B:** Residential proxy service (expensive, ~$15+/month)
- **Option C:** Rotating datacenter proxies (cheap but higher block rate)
- **Recommendation:** Start with Option A (2-3 Squid droplets = $8-12/month)

#### 3. Infrastructure as Code
- **Tool:** Terraform
- **Provider:** DigitalOcean
- **One-command deploy:** `terraform apply`
- **Outputs:** API endpoint URL, proxy IPs

---

## Technical Deep Dive

### How YouTube Transcripts Work

YouTube embeds transcript data in the page in two ways:

1. **Initial Page Data:** JSON blob in `ytInitialPlayerResponse`
2. **Transcript API:** Hidden endpoint at `youtube.com/api/timedtext`

The timedtext API returns captions in various formats (srv1, srv2, srv3, json3, vtt).

**Key insight:** The transcript API doesn't require auth - it just needs:
- Valid video ID
- Caption track parameters (from initial page data)
- Browser-like request headers
- Non-blocked IP

### Scraping Strategy

```
1. GET video page with browser-like headers
   └─▶ Rotate through proxy pool
   └─▶ Extract ytInitialPlayerResponse JSON

2. Parse caption track info from JSON
   └─▶ Get track URL, language, kind (asr = auto-generated)

3. GET caption track from timedtext API
   └─▶ Through same proxy
   └─▶ Parse VTT/SRT response

4. Clean transcript text
   └─▶ Remove timestamps, formatting
   └─▶ Deduplicate repeated lines

5. Summarize with LLM
   └─▶ OpenRouter API (Gemini Flash)
```

### Headers That Matter

```http
User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8
Accept-Language: en-US,en;q=0.5
Accept-Encoding: gzip, deflate, br
Connection: keep-alive
Upgrade-Insecure-Requests: 1
Sec-Fetch-Dest: document
Sec-Fetch-Mode: navigate
Sec-Fetch-Site: none
Sec-Fetch-User: ?1
```

### Proxy Rotation Logic

```go
type ProxyPool struct {
    proxies []string
    current int
    mutex   sync.Mutex
}

func (p *ProxyPool) Next() string {
    p.mutex.Lock()
    defer p.mutex.Unlock()
    proxy := p.proxies[p.current]
    p.current = (p.current + 1) % len(p.proxies)
    return proxy
}

// On failure, mark proxy as "cooling down"
// Retry with different proxy
// After N failures, alert
```

---

## Cost Analysis

### Option A: Minimal (Self-hosted proxies)

| Component | Spec | Monthly Cost |
|-----------|------|--------------|
| Scraper droplet | $6 droplet (1GB RAM) | $6 |
| Proxy droplet x2 | $4 droplet each | $8 |
| **Total** | | **$14/month** |

### Option B: Single droplet (if detection is loose)

| Component | Spec | Monthly Cost |
|-----------|------|--------------|
| Scraper droplet | $6 droplet | $6 |
| **Total** | | **$6/month** |

### Option C: With residential proxy service

| Component | Spec | Monthly Cost |
|-----------|------|--------------|
| Scraper droplet | $6 droplet | $6 |
| Bright Data/Oxylabs | Pay-per-GB (~$15 min) | $15+ |
| **Total** | | **$21+/month** |

**Recommendation:** Start with Option B, add proxy droplets if needed.

---

## Implementation Plan

### Phase 0: Foundation (v0.0) ✓
- [x] Add HTTP server with `/health`, `/transcript`, `/summarize` endpoints (Gap 11)
- [x] Configure server timeouts and graceful shutdown (Gap 11)
- [x] Add structured logging with slog (Gap 14)
- [x] Add service-level rate limiting (Gap 12)
- [x] Migrate cache from filesystem to SQLite with proper config (Gap 13, Gap 18)
- [x] Update URL parser for shorts/live/mobile formats (Gap 17)
- [x] Add `serve` command to CLI
- [x] Create Dockerfile (Gap 20)

### Phase 1: Core Scraper (v0.1) ✓
- [x] Update ytsummary to use direct HTTP scraping instead of yt-dlp
- [x] Use YouTube innertube API with ANDROID client (bypasses age restrictions)
- [x] Handle edge cases (age-restricted, private, live streams)
- [x] Test locally - works for all video types
- [x] Document approach in README

**Datacenter Testing Results:**
- ✅ Works locally (residential IP) for all tested videos
- ⚠️ From datacenter: Some videos work (Rick Astley), others return "LOGIN_REQUIRED" with "Sign in to confirm you're not a bot"
- YouTube detects datacenter IPs and applies stricter bot checks inconsistently
- Different innertube clients (WEB, IOS, TVHTML5) don't bypass this - it's IP-based

### Phase 2: Proxy Support (v0.2)
- [ ] Add proxy configuration to scraper (HTTP_PROXY env var)
- [ ] Implement proxy rotation with fallback
- [ ] Add retry logic with exponential backoff
- [ ] Test options:
  - [ ] Self-hosted Squid proxy on different datacenter IP (likely blocked)
  - [ ] Residential proxy service (higher cost, more reliable)
  - [ ] Hybrid: try direct first, fall back to proxy

**Key insight from Phase 1 testing:** Datacenter IP blocking is IP-based, not client-based. Different innertube clients don't help - need residential IPs or accept partial success rate.

### Phase 3: Infrastructure (v0.3)
- [ ] Create DigitalOcean container registry via Terraform (Gap 15)
- [ ] Terraform module for scraper droplet
- [ ] Terraform module for proxy droplets
- [ ] One-command deployment
- [ ] Output API endpoint

### Phase 4: Atlas Integration (v1.0)
- [ ] Deploy to DigitalOcean
- [ ] Update Atlas skill to call scraper API
- [ ] Implement health check monitoring in Atlas (Gap 16)
- [ ] Add to exec-approvals
- [ ] Test end-to-end via WhatsApp

### Phase 5: Polish (v1.1)
- [ ] Monitoring/alerting for failures
- [ ] Usage stats
- [ ] README with architecture diagram
- [ ] Blog post / portfolio write-up

### Deferred
- [ ] Summary caching (Gap 19) - revisit if LLM costs become significant

---

## Repository Structure

```
ytsummary/
├── cmd/
│   └── ytsummary/
│       └── main.go           # CLI entry point
├── internal/
│   ├── scraper/
│   │   ├── scraper.go        # YouTube page scraping
│   │   ├── transcript.go     # Caption extraction
│   │   └── headers.go        # Browser-like headers
│   ├── proxy/
│   │   ├── pool.go           # Proxy rotation
│   │   └── health.go         # Proxy health checks
│   └── summarize/
│       └── llm.go            # LLM integration
├── api/
│   └── server.go             # HTTP API for Atlas
├── terraform/
│   ├── main.tf               # Main infrastructure
│   ├── variables.tf          # Configurable vars
│   ├── outputs.tf            # API endpoint, etc.
│   └── modules/
│       ├── scraper/          # Scraper droplet
│       └── proxy/            # Proxy droplet(s)
├── scripts/
│   └── deploy.sh             # Deployment helper
├── .env.example
├── SPEC.md                   # This document
└── README.md                 # Public documentation
```

---

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| YouTube changes page structure | Medium | High | Abstract parsing, update quickly |
| All proxy IPs blocked | Low | High | Use residential proxies as fallback |
| Rate limited | Medium | Medium | Implement backoff, spread requests |
| LLM costs spike | Low | Low | Use cheap model (Gemini Flash), cache results |

---

## Open Questions

1. **Headless browser vs direct HTTP?**
   - Headless (Playwright/Rod) = more reliable, heavier
   - Direct HTTP = lighter, may get blocked easier
   - **Decision:** Start with direct HTTP, upgrade if needed

2. **How many proxy IPs needed?**
   - Depends on usage volume
   - Start with 2, monitor block rate
   - **Decision:** Start small, scale as needed

3. **Cache transcripts permanently or with TTL?**
   - Transcripts rarely change
   - **Decision:** Cache permanently, manual invalidation

4. **API authentication for scraper service?**
   - Simple API key sufficient for personal use
   - **Decision:** API key in header, rotate if leaked

---

## Gaps to Address

### Gap 1: API Design
**Status:** DONE

**Endpoints:**
```
GET  /health              → Health check
POST /transcript          → Fetch transcript only
POST /summarize           → Fetch transcript + summarize
```

**Request format:**
```json
{
  "url": "https://youtube.com/watch?v=xxxxx",
  "language": "en"  // optional, default "en"
}
```

**Response format (success):**
```json
{
  "video_id": "xxxxx",
  "title": "Video Title",
  "transcript": "...",      // only for /transcript
  "summary": "...",         // only for /summarize
  "cached": true,
  "duration_ms": 1234
}
```

**Response format (error):**
```json
{
  "error": "no_captions",
  "message": "This video has no captions available",
  "video_id": "xxxxx"
}
```

**Error codes:**
| Code | Meaning |
|------|---------|
| `no_captions` | Video has no captions/subtitles |
| `video_unavailable` | Private, deleted, or region-locked |
| `age_restricted` | Requires sign-in |
| `rate_limited` | Too many requests, try later |
| `scrape_failed` | YouTube blocked or changed structure |
| `llm_error` | Summarization failed |

**Timeout handling:**
- Request timeout: 60 seconds
- If exceeded, return `504` with partial progress info

---

### Gap 2: Caching Strategy
**Status:** DONE

**Storage:**
- [ ] File system (simple, `/var/cache/ytsummary/`)
- [x] SQLite (queryable, single file)
- [ ] Redis (fast, but extra infra)

**Decision:** SQLite - single file, queryable, easy backup, no extra infra

**Persistence:** Accept cache loss on redeploy - simpler, cache rebuilds organically. Not worth the extra cost/complexity for personal use.

**Cache key format:**
```
{video_id}_{language}.json
```

**Cached data structure:**
```json
{
  "video_id": "xxxxx",
  "title": "Video Title",
  "language": "en",
  "transcript": "...",
  "fetched_at": "2026-02-05T12:00:00Z"
}
```

**Cache limits:**
- Max entries: TBD (1000? 10000?)
- Max disk usage: TBD (1GB?)
- Eviction policy: LRU or oldest-first

**Persistent storage:**
- DigitalOcean Volumes ($0.10/GB/month) for persistence across droplet rebuilds
- Or: Accept cache loss on redeploy (simpler)

---

### Gap 3: Secrets Management
**Status:** DONE

**Secrets inventory:**
| Secret | Used By | Location |
|--------|---------|----------|
| OpenRouter API key | Scraper | `terraform.tfvars` → env var |
| Proxy auth (user:pass) | Scraper → Proxy | `terraform.tfvars` → env var |
| Scraper API key | Atlas → Scraper | `terraform.tfvars` → env var |
| DO API token | Terraform | `terraform.tfvars` |

**Options:**
- [x] Environment variables (simple, passed via Terraform)
- [ ] DigitalOcean Secrets (native, but limited)
- [ ] HashiCorp Vault (overkill for personal project)
- [ ] Encrypted `.env` in repo + decrypt on deploy

**Decision:** Terraform with gitignored `terraform.tfvars` for secret values. TF state kept local (not committed). Secrets injected as environment variables on droplet.

---

### Gap 4: Deployment Runtime
**Status:** DONE

**Options:**
- [ ] Systemd service (direct binary on droplet)
- [ ] Docker container (isolated, consistent)
- [x] Docker + systemd (container managed by systemd)

**Decision:** Docker + systemd
- Build Go binary in Docker (handles cross-compilation)
- Push to personal DO container registry
- Systemd unit pulls and runs the container
- Easy rollback via image tags

**Pre-requisite:** Create personal DigitalOcean Container Registry
- Create new registry: `registry.digitalocean.com/alrobwilloliver/` or similar
- Add registry creation to Terraform, or create manually first

**Requirements:**
- Auto-restart on crash
- Start on boot
- Log to journald or file
- Graceful shutdown

**Log rotation:**
- journald handles automatically, OR
- logrotate config for file-based logs

---

### Gap 5: Edge Cases
**Status:** DONE

| Case | Detection | Response |
|------|-----------|----------|
| No captions | Empty `captionTracks` in response | Return `no_captions` error |
| Non-English only | No `en` track in `captionTracks` | Return first available language (see below) |
| Age-restricted | `playabilityStatus.reason` contains age message | Return `age_restricted` error |
| Private video | `playabilityStatus.status` = "UNPLAYABLE" | Return `video_unavailable` error |
| Live stream | `isLive` = true | Return `live_stream` error |
| Very long video (4h+) | Transcript > 100k chars | Chunk for summarization (already handled) |

**Language fallback strategy:**
- Priority: `en` → `en-US` → `en-GB` → first available
- Return whichever language is available - LLM can translate during summarization if needed
- Include `language` field in response so Atlas knows what was returned
- User can prompt Atlas to translate if the source language isn't English

---

### Gap 6: Monitoring & Alerting
**Status:** DONE

**Level:** Basic (appropriate for personal use)

**Implementation:**
- [x] `/health` endpoint returns service status
- [x] Atlas checks health endpoint during heartbeats
- [x] Atlas messages you if service is down
- [x] Logs available on droplet for debugging

**Health endpoint response:**
```json
{
  "status": "ok",
  "cache_entries": 42,
  "uptime_seconds": 86400,
  "last_success": "2026-02-05T12:00:00Z"
}
```

**No external monitoring services needed** - Atlas is the monitor.

---

### Gap 7: Testing Strategy
**Status:** DONE

**Level:** Moderate

**Unit tests:**
- Parser tests with recorded YouTube responses (fixtures)
- Cache tests with mock storage
- Proxy rotation logic tests

**Test fixtures:**
- Save sample `ytInitialPlayerResponse` JSON for various cases:
  - Normal video with captions
  - Video without captions
  - Age-restricted video
  - Private video

**Integration tests:**
- Live YouTube fetch (sparingly, tagged to skip in CI)
- LLM summarization with mock API

**CI/CD:**
- Run unit tests on push
- Skip live integration tests in CI

---

### Gap 8: Rate Limiting Strategy
**Status:** DONE (with unknowns acknowledged)

**The reality:** Too many unknowns about YouTube's blocking behavior.
- Will different DO IPs even help? YouTube may block entire datacenter ranges.
- Spinning up new proxies dynamically adds complexity for uncertain benefit.
- Won't know until we try.

**Progressive approach:**
1. **Start with no proxies** - direct requests from scraper droplet
2. **If blocked:** Add 1-2 proxy droplets, test if different DO IPs help
3. **If still blocked:** Consider residential proxy service as fallback
4. **If that fails:** Accept that server-side scraping may not be viable

**Basic rate limiting (regardless of proxy setup):**

| Setting | Value |
|---------|-------|
| Requests per minute (total) | 10 |
| Backoff sequence | 2s → 4s → 8s |
| Max retries | 3 |
| Cool-down after block | 30 min |

**If all sources exhausted:**
- Return `rate_limited` error
- Atlas tells user: "YouTube is blocking us, try again later"

**Key insight:** Build the proxy rotation code, but don't deploy proxy droplets until we know we need them. Start minimal.

---

### Gap 9: Failure Modes
**Status:** DONE

| Failure | Detection | API Response | Atlas Interpretation |
|---------|-----------|--------------|---------------------|
| Scraper service down | Connection refused | (no response) | "YouTube summary service is temporarily unavailable" |
| All proxies blocked | All return 429/403 | `rate_limited` | "YouTube is rate limiting us, try again in 30 min" |
| YouTube structure changed | Parse error on expected fields | `scrape_failed` | "YouTube changed something, Alan needs to update the scraper" |
| LLM API down | API timeout/error | Return transcript only | "Got the transcript but couldn't summarize it: [transcript]" |
| LLM rate limited | 429 from OpenRouter | `llm_error` | "Summarization failed, here's the raw transcript" |

**Graceful degradation:**
- If summarization fails → still return transcript
- If proxy fails → try next, then direct
- API returns error codes, Atlas translates to friendly messages

---

### Gap 10: ytInitialPlayerResponse Parsing
**Status:** DONE

**Location in page:**
```javascript
var ytInitialPlayerResponse = {...};
// OR
ytInitialPlayerResponse = {...};
```

**Extraction regex:**
```go
re := regexp.MustCompile(`ytInitialPlayerResponse\s*=\s*(\{.+?\});`)
```

**Key JSON paths:**
```
.captions.playerCaptionsTracklistRenderer.captionTracks[]
  .baseUrl        → URL to fetch captions
  .languageCode   → "en", "es", etc.
  .kind           → "asr" (auto-generated) or empty (manual)
  .name.simpleText → "English (auto-generated)"

.videoDetails.videoId
.videoDetails.title
.videoDetails.lengthSeconds

.playabilityStatus.status → "OK", "UNPLAYABLE", "LOGIN_REQUIRED"
.playabilityStatus.reason → Human-readable error
```

**Caption URL format:**
```
https://www.youtube.com/api/timedtext?v=VIDEO_ID&...params...&fmt=vtt
```

**Decision:** Use VTT format - already have a working parser from the yt-dlp version.

---

### Gap 11: HTTP Server Configuration
**Status:** DONE

Server implemented with all configuration settings.

**Implementation:** See `server.go:17-24` for timeout constants, `server.go:68-120` for server setup and graceful shutdown.

| Setting | Value | Rationale |
|---------|-------|-----------|
| Request body size limit | 1KB | Only accepting JSON with URL + language |
| Read timeout | 5s | Time to read full request |
| Write timeout | 120s | Summarization can take time |
| Idle timeout | 60s | Keep-alive connection limit |
| Graceful shutdown timeout | 30s | Time to finish in-flight requests |

**Endpoints:**
- `GET /health` - Health check with cache stats and uptime
- `POST /transcript` - Fetch transcript only
- `POST /summarize` - Fetch transcript and summarize

**CLI:** `ytsummary serve --addr :8080 --server-api-key SECRET`

**Auth:** X-API-Key header or Bearer token supported.

---

### Gap 12: API Rate Limiting (Service Protection)
**Status:** DONE

Protect the service from abuse (distinct from YouTube rate limiting in Gap 8).

**Implementation:** See `ratelimit.go` - per-IP rate limiting using `golang.org/x/time/rate`.

| Setting | Value |
|---------|-------|
| Requests per minute per IP | 30 |
| Burst allowance | 5 |
| Stale entry cleanup | 5 minutes |

**Features:**
- Per-IP tracking with automatic cleanup of stale entries
- Supports X-Forwarded-For and X-Real-IP headers for reverse proxy setups
- Returns 429 with Retry-After header when rate limited
- Health endpoint excluded from rate limiting

---

### Gap 13: SQLite Concurrency
**Status:** DONE

SQLite configuration for safe concurrent access.

**Implementation:** See `cache.go:38-47` for DSN configuration and connection settings.

| Setting | Value | Purpose |
|---------|-------|---------|
| `_busy_timeout` | 5000ms | Wait for locks instead of failing |
| `_journal_mode` | WAL | Better concurrent read/write |
| `_synchronous` | NORMAL | Balance safety/performance |
| Max open connections | 1 | SQLite handles one writer at a time |

**Schema:** See `cache.go:50-59`

---

### Gap 14: Logging Strategy
**Status:** DONE

**Implementation:** See `logging.go` for structured JSON logging using `log/slog`.

**Features:**
- Structured JSON output to stdout (Docker captures it)
- Log levels: DEBUG, INFO, WARN, ERROR
- Request logging middleware captures method, path, status, duration, IP, video_id, cache_hit
- Server lifecycle events logged (start, shutdown)
- Cache operations logged at DEBUG level
- Errors and warnings logged appropriately

**Sample log output:**
```json
{"time":"2026-02-06T12:00:00Z","level":"INFO","msg":"request completed","method":"POST","path":"/summarize","status":200,"duration_ms":1234,"ip":"192.168.1.1","video_id":"dQw4w9WgXcQ","cache_hit":false}
```

---

### Gap 15: Container Registry Setup
**Status:** TODO

**Pre-requisite for Phase 4.** Add to Implementation Plan.

**Manual steps (one-time):**
1. Create registry: `doctl registry create alroliver-registry --region nyc3`
2. Note: Free tier allows 500MB storage, 5GB transfer/month

**Or via Terraform:**
```hcl
resource "digitalocean_container_registry" "main" {
  name                   = "alroliver"
  subscription_tier_slug = "starter"  # Free tier
  region                 = "nyc3"
}
```

**Decision:** Create via Terraform for reproducibility.

---

### Gap 16: Health Check Definition
**Status:** TODO

Clarify what "healthy" means:

| Check | Healthy | Unhealthy |
|-------|---------|-----------|
| Service responding | `/health` returns 200 | Connection refused |
| Database accessible | Can query SQLite | Query fails |
| Recent success | `last_success` < 1 hour ago | `last_success` > 1 hour ago (WARN only) |

**Health response:**
```json
{
  "status": "ok",           // or "degraded", "unhealthy"
  "cache_entries": 42,
  "uptime_seconds": 86400,
  "last_success": "2026-02-05T12:00:00Z",
  "last_success_age_seconds": 300
}
```

**Atlas behavior:**
- `status: ok` → Service is fine
- `status: degraded` → Service works but `last_success` is stale (>1h)
- `status: unhealthy` → Database error or critical failure
- Connection refused → Alert Alan

---

### Gap 17: Additional URL Formats
**Status:** DONE

Updated `extractVideoID` to handle all formats:

| Format | Example |
|--------|---------|
| Shorts | `youtube.com/shorts/VIDEO_ID` |
| Live | `youtube.com/live/VIDEO_ID` |
| With timestamp | `youtube.com/watch?v=VIDEO_ID&t=123` |
| Mobile | `m.youtube.com/watch?v=VIDEO_ID` |

**Implementation:** See `transcript.go:14-44` and `transcript_test.go` for tests.

---

### Gap 18: Cache Key with Language
**Status:** DONE

Cache now uses composite primary key (video_id, language).

**Implementation:** See `cache.go:77-104` for `getCachedTranscript(videoID, language)` and `cache.go:107-124` for `cacheTranscript(videoID, language, title, transcript)`.

**Behavior:**
1. Request comes in for video X, language "en"
2. Check cache for (X, "en")
3. If miss, fetch transcript
4. Cache stores (X, "en", transcript)
5. Later request for (X, "es") is a separate cache entry

**CLI flag:** `--lang` flag added to specify preferred language (default: "en").

---

### Gap 19: Summary Caching
**Status:** TODO

**Question:** Should summaries be cached?

**Arguments for:**
- Saves LLM API costs on repeated requests
- Faster response for cached videos

**Arguments against:**
- Summaries depend on prompt/model (may want to regenerate)
- Transcripts are more stable than "good summaries"
- Storage overhead

**Decision:** Don't cache summaries initially.
- Transcripts rarely change, worth caching
- Summaries may need iteration (prompt tuning)
- LLM costs are low with Gemini Flash (~$0.001 per summary)
- Can add summary caching later if costs become an issue

---

### Gap 20: Dockerfile
**Status:** DONE

**Implementation:** See `Dockerfile` and `.dockerignore`.

**Features:**
- Multi-stage build (golang:1.24-alpine → alpine:3.21)
- CGO enabled for SQLite support
- Final image size: ~43MB
- Built-in health check (wget to /health)
- Cache directory at /app/cache

**Build & Run:**
```bash
docker build -t ytsummary .
docker run -p 8080:8080 -e YTSUMMARY_API_KEY=xxx -e YTSUMMARY_SERVER_API_KEY=yyy ytsummary
```

---

## Existing Infrastructure (Reusable)

Existing Squid proxy setup available for reuse:

- **Port:** 8888 with basic auth
- **Features:** Hides Via header, deletes forwarded-for (anonymizing)
- **Deploy script:** Handles versioning, tagging, push to DO registry

This can be reused directly for the proxy pool - just spin up multiple droplets running the same image.

---

## References

- [yt-dlp YouTube extractor source](https://github.com/yt-dlp/yt-dlp/blob/master/yt_dlp/extractor/youtube.py)
- [youtube-transcript-api (Python)](https://github.com/jdepoix/youtube-transcript-api)
- [Terraform DigitalOcean Provider](https://registry.terraform.io/providers/digitalocean/digitalocean/latest/docs)

---

## Changelog

- **2026-02-06:** Added Gaps 11-20 (server config, rate limiting, SQLite, logging, registry, health checks, URL formats, cache keys, summary caching, Dockerfile). Added Phase 0 to implementation plan.
- **2026-02-05:** Initial spec created
