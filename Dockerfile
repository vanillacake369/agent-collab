# =============================================================================
# Multi-stage Dockerfile for agent-collab
# =============================================================================

# -----------------------------------------------------------------------------
# Build stage
# -----------------------------------------------------------------------------
FROM golang:latest AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /build

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o /app/agent-collab ./src/cmd/agent-collab

# -----------------------------------------------------------------------------
# Runtime stage
# -----------------------------------------------------------------------------
FROM alpine:3.21

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -g '' agentcollab

# Copy binary from builder
COPY --from=builder /app/agent-collab /app/agent-collab

# Create data directory with proper permissions
RUN mkdir -p /data && chown agentcollab:agentcollab /data

# Switch to non-root user
USER agentcollab

WORKDIR /app

# Expose P2P port
EXPOSE 9000

# Health check
HEALTHCHECK --interval=5s --timeout=3s --start-period=5s --retries=3 \
    CMD /app/agent-collab status || exit 1

ENTRYPOINT ["/app/agent-collab"]
CMD ["daemon", "run"]
