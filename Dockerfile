# Multi-stage Dockerfile for omniagent
# Builds standalone with no local replace directives.
#
# Build:
#   docker build --platform linux/amd64 -t ghcr.io/plexusone/omniagent:smoke .
#
# Run (requires env vars for LLM + Discord):
#   docker run -p 8080:8080 \
#     -e OMNIAGENT_GATEWAY_ADDRESS=0.0.0.0:8080 \
#     -e OMNIAGENT_AGENT_PROVIDER=anthropic \
#     -e OMNIAGENT_AGENT_MODEL=claude-sonnet-5 \
#     -e ANTHROPIC_API_KEY=... \
#     -e DISCORD_BOT_TOKEN=... \
#     ghcr.io/plexusone/omniagent:smoke
#
# IMPORTANT: Single-replica deployment required for Discord
# Discord uses a WebSocket gateway connection that is stateful. Running
# multiple replicas will cause each to receive and process messages,
# resulting in duplicate (double) answers. Set OMNIAGENT_REPLICAS=1 or
# ensure only one container instance runs when Discord is enabled.

# ---------------------------------------------------------------------------
# Build stage
# ---------------------------------------------------------------------------
FROM golang:1.26-alpine AS builder

WORKDIR /build

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o omniagent \
    ./cmd/omniagent

# ---------------------------------------------------------------------------
# Runtime stage
# ---------------------------------------------------------------------------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

RUN adduser -D -u 1000 omniagent \
    && mkdir -p /data /opt/omniagent \
    && chown -R omniagent:omniagent /data /opt/omniagent

WORKDIR /opt/omniagent

COPY --from=builder /build/omniagent /opt/omniagent/omniagent

USER omniagent

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q --spider http://localhost:8080/api/health || exit 1

ENTRYPOINT ["/opt/omniagent/omniagent"]
CMD ["gateway", "run"]
