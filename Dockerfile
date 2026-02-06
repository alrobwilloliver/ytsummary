# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies for SQLite (CGO)
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with CGO enabled for SQLite
RUN CGO_ENABLED=1 go build -o ytsummary .

# Runtime stage
FROM alpine:3.21

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite-libs

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/ytsummary .

# Create cache directory
RUN mkdir -p /app/cache

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:8080/health || exit 1

# Run server
ENTRYPOINT ["./ytsummary"]
CMD ["serve", "--addr", ":8080", "--cache-dir", "/app/cache"]
