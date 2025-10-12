# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X 'github.com/SergeiSkv/AiBsCleaner/version.Version=docker' -X 'github.com/SergeiSkv/AiBsCleaner/version.CommitHash=$(git rev-parse HEAD 2>/dev/null || echo unknown)' -X 'github.com/SergeiSkv/AiBsCleaner/version.BuiltAt=$(date -u +%Y-%m-%dT%H:%M:%SZ)'" \
    -o aibscleaner \
    .

# Run tests
RUN go test -short ./...

# Final stage
FROM alpine:latest

LABEL maintainer="Sergei Skoredin"
LABEL description="Performance-focused static analyzer for Go"
LABEL version="1.0.0"

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates git

WORKDIR /workspace

# Copy binary from builder
COPY --from=builder /build/aibscleaner /usr/local/bin/aibscleaner

# Create non-root user
RUN addgroup -S aibscleaner && adduser -S aibscleaner -G aibscleaner
USER aibscleaner

ENTRYPOINT ["aibscleaner"]
CMD ["--help"]
