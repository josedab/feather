# Build stage
# Images are pinned with sha256 digest for reproducible builds
# To update: check https://hub.docker.com/_/golang/tags for latest digests
FROM golang:1.22-alpine@sha256:bd3cd9eea80c49c32e9a19cbeb41744bc8434a26f8f9d416f540df1866876bf1 AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -extldflags '-static'" \
    -o /feather \
    ./cmd/feather

# Final stage - minimal image
# Images are pinned with sha256 digest for reproducible builds
# To update: check https://hub.docker.com/_/alpine/tags for latest digests
FROM alpine:3.19@sha256:45eeb55d6698849eb12a02d3e9a323e3d8e656882ef4ca542d1dda0274231e84

# Install ca-certificates for HTTPS and tzdata for timezones
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 feather && \
    adduser -u 1000 -G feather -s /bin/sh -D feather

# Create data and config directories
RUN mkdir -p /var/lib/feather/data /etc/feather && \
    chown -R feather:feather /var/lib/feather /etc/feather

# Copy binary from builder with correct ownership
COPY --from=builder --chown=feather:feather /feather /usr/local/bin/feather

# Copy default config with correct ownership
# Note: COPY will fail if source doesn't exist; ensure configs/feather.yaml exists
COPY --from=builder --chown=feather:feather /app/configs/feather.yaml /etc/feather/feather.yaml

# Switch to non-root user
USER feather

# Set working directory
WORKDIR /var/lib/feather

# Expose ports
# HTTP serving
EXPOSE 8080
# gRPC serving
EXPOSE 50051
# HTTP ingestion
EXPOSE 8081
# Prometheus metrics
EXPOSE 9090

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Default environment variables
ENV FEATHER_HTTP_PORT=8080 \
    FEATHER_GRPC_PORT=50051 \
    FEATHER_PROMETHEUS_PORT=9090 \
    FEATHER_HTTP_INGESTION_PORT=8081 \
    FEATHER_HOT_MAX_MEMORY=4GB \
    FEATHER_WARM_PATH=/var/lib/feather/data \
    FEATHER_LOG_LEVEL=info \
    FEATHER_LOG_FORMAT=json

# Run the binary
ENTRYPOINT ["/usr/local/bin/feather"]

# Default command (can be overridden)
CMD ["-config", "/etc/feather/feather.yaml"]
