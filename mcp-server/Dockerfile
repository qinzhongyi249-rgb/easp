# ============================================
# EASP Python MCP Server - Dockerfile
# ============================================
FROM python:3.12-slim

LABEL org.opencontainers.image.title="EASP MCP Server"
LABEL org.opencontainers.image.description="Enterprise API-to-MCP Gateway - Python MCP Server"
LABEL org.opencontainers.image.source="https://github.com/qinzhongyi249-rgb/easp"
LABEL org.opencontainers.image.licenses="AGPL-3.0"

WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements first for better caching
COPY mcp-server/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy application code
COPY mcp-server/app/ ./app/
COPY mcp-server/mcp_config.json ./mcp_config.json

# Create non-root user
RUN useradd --create-home --shell /bin/bash mcp && \
    chown -R mcp:mcp /app
USER mcp

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:${PORT:-9000}/sse || exit 1

EXPOSE ${PORT:-9000}

# Start MCP Server
CMD ["sh", "-c", "python3 -m app.main"]
