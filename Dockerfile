# ============================================
# EASP Go MCP Tools - Dockerfile
# ============================================
FROM golang:1.23-alpine AS builder

LABEL org.opencontainers.image.title="EASP MCP Tools"
LABEL org.opencontainers.image.description="Enterprise API-to-MCP Gateway - Go MCP Tools"
LABEL org.opencontainers.image.source="https://github.com/qinzhongyi249-rgb/easp"
LABEL org.opencontainers.image.licenses="AGPL-3.0"

WORKDIR /build

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build MCP test tool
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/mcp-test ./cmd/mcp-test

# Build MCP E2E tool
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/mcp-e2e ./cmd/mcp-e2e

# ============================================
# Runtime stage
# ============================================
FROM alpine:3.21

RUN apk add --no-cache ca-certificates curl

COPY --from=builder /out/mcp-test /usr/local/bin/mcp-test
COPY --from=builder /out/mcp-e2e /usr/local/bin/mcp-e2e

USER nobody

ENTRYPOINT ["/usr/local/bin/mcp-test"]
