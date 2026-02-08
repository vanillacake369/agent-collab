# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o /agent-collab ./cmd/agent-collab

# Runtime stage
FROM alpine:3.21

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -g '' agentcollab

# Copy binary from builder
COPY --from=builder /agent-collab /usr/local/bin/agent-collab

# Switch to non-root user
USER agentcollab

# Create data directory
RUN mkdir -p /home/agentcollab/.agent-collab

# Expose P2P port
EXPOSE 4001

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD agent-collab status || exit 1

ENTRYPOINT ["agent-collab"]
CMD ["daemon", "run"]
