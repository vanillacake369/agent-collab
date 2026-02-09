# Runtime stage - GoReleaser provides pre-built binary
FROM alpine:3.21

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -g '' agentcollab

# Copy binary from GoReleaser build
COPY agent-collab /usr/local/bin/agent-collab

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
