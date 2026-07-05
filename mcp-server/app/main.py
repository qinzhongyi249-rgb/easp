from fastapi import FastAPI, Request, Depends
from starlette.responses import Response, StreamingResponse
from app.routers import oa, time_tool, travel, wecom
from app.mcp_server import mcp_server as mcp_app
from app.dependencies import verify_api_key
from mcp.server.sse import SseServerTransport
import uvicorn
import time
from app.logger import logger
from starlette.middleware.base import BaseHTTPMiddleware

# --- Define FastAPI App (Unified) ---

app = FastAPI(
    title="OA & WeCom Integration API (REST + MCP)",
    description="""
    This unified service provides both standard RESTful APIs and an MCP (Model Context Protocol) SSE endpoint.
    
    ## Authentication
    All REST API endpoints require an API Token.
    Header: `X-API-Token: joyyunyou-test-token`
    
    ## REST API
    Standard HTTP endpoints for external integrations.
    
    ## MCP API (SSE)
    - Endpoint: `/sse`
    - Message Endpoint: `/messages`
    
    Connect your AI Agent to `http://localhost:8000/sse` to use the tools defined in this service.
    """,
    version="2.0.0",
    dependencies=[Depends(verify_api_key)] # Apply global auth
)

# Initialize SSE Transport globally to share session state
# Use a path under /sse to ensure it works behind reverse proxies that only proxy /sse
sse_transport = SseServerTransport("/sse/messages")

# --- Middleware for Logging ---
# Using a pure ASGI middleware to avoid BaseHTTPMiddleware issues with custom ASGI responses (like SSE)

class LoggingMiddleware:
    def __init__(self, app):
        self.app = app

    async def __call__(self, scope, receive, send):
        if scope["type"] != "http":
            await self.app(scope, receive, send)
            return

        start_time = time.time()
        
        # Extract request info
        method = scope.get("method", "UNKNOWN")
        path = scope.get("path", "UNKNOWN")
        query_string = scope.get("query_string", b"").decode("utf-8")
        
        logger.info(f"Incoming Request: {method} {path} (Query: {query_string})")
        
        # Note: We cannot easily log headers or body in pure ASGI without significant overhead/complexity
        # especially the body which must be buffered.
        # For headers, we can iterate scope['headers'].
        try:
            headers = dict(scope.get("headers", []))
            # Decode bytes to strings for nicer logging
            decoded_headers = {k.decode('utf-8'): v.decode('utf-8') for k, v in headers.items()}
            logger.debug(f"Request Headers: {decoded_headers}")
        except Exception:
            pass

        # To capture status code, we wrap 'send'
        async def wrapped_send(message):
            if message["type"] == "http.response.start":
                status_code = message.get("status")
                process_time = time.time() - start_time
                logger.info(f"Response Status: {status_code} (Time: {process_time:.4f}s)")
            
            # We skip response body logging in pure ASGI to avoid breaking streaming/SSE
            # Logging response body requires intercepting chunks, buffering, logging, and re-emitting.
            # Given the stability issues we've seen, it's safer to skip it for now or implement it very carefully.
            # If strictly needed, we can add it later for non-SSE paths.
            
            await send(message)

        try:
            await self.app(scope, receive, wrapped_send)
        except Exception as e:
            logger.error(f"Request failed: {e}", exc_info=True)
            raise e

app.add_middleware(LoggingMiddleware)

# Mount Routers
app.include_router(oa.router)
app.include_router(wecom.router)
app.include_router(time_tool.router)
app.include_router(travel.router)

# Mount MCP Message Endpoint
@app.post("/sse/messages")
async def handle_messages(request: Request):
    # sse_transport.handle_post_message expects the ASGI scope, receive, and send callables
    # However, in FastAPI/Starlette, request._send is an internal attribute and might not be reliable or safe to use directly
    # Also, handle_post_message is designed to be an ASGI app itself.
    # To properly bridge it, we should await it directly with the ASGI interface.
    
    # But wait, request._send IS the ASGI send callable in Starlette requests.
    # The error "Unexpected ASGI message 'http.response.start' sent, after response already completed"
    # suggests that FastAPI might be trying to send its own response AFTER handle_post_message has already sent one.
    # Because handle_post_message writes directly to the ASGI send channel.
    
    # When a FastAPI path operation function returns (or implicitly returns None), FastAPI tries to send a response.
    # If handle_post_message has already completed the response cycle, FastAPI's attempt will fail.
    
    # Solution: We need to return a Response object that tells FastAPI "the response is already handled" or
    # return a custom Response that executes the ASGI app.
    
    # A cleaner way in FastAPI to wrap an ASGI app is to use a generic route or just return a Response
    # that delegates to the ASGI app.
    
    # Let's try to return a Response that wraps the ASGI app execution.
    
    class ASGIResponse(Response):
        async def __call__(self, scope, receive, send):
            await sse_transport.handle_post_message(scope, receive, send)
            
    return ASGIResponse()

# Mount MCP SSE Endpoint
@app.get("/sse")
@app.post("/sse")
async def handle_sse(request: Request):
    """
    Handle SSE connections for MCP.
    """
    logger.debug(f"mcp_app type: {type(mcp_app)}")
    logger.debug(f"sse_transport type: {type(sse_transport)}")
    
    # Access the underlying Server instance from FastMCP
    try:
        server_instance = mcp_app._mcp_server
    except AttributeError:
        # Fallback if _mcp_server doesn't exist, maybe it's just `server`?
        try:
            server_instance = mcp_app.server
        except AttributeError:
             # Last resort: FastMCP might be the server itself (if it inherits)
             server_instance = mcp_app

    async with sse_transport.connect_sse(request.scope, request.receive, request._send) as (read_stream, write_stream):
        await server_instance.run(read_stream, write_stream, server_instance.create_initialization_options())

@app.get("/")
def read_root():
    """Root endpoint to verify service status."""
    return {"message": "OA API Service is running (REST + MCP)"}

from app.config import PORT

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=PORT)
