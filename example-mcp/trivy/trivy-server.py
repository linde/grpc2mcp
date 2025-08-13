
"""

curl -i -X POST -H "Accept: application/json, text/event-stream" -H "Content-Type: application/json" -d '{ "jsonrpc": "2.0", "id": "1", "method": "initialize", "params": { "protocolVersion": "2025-06-18", "capabilities": {}, "clientInfo": { "name": "curl", "version": "1.0" } } }' http://localhost:8888/mcp/

# get the mcp-session-id header and export it as MCP_SESSION_ID

curl -i -X POST -H "Accept: application/json, text/event-stream" -H "Content-Type: application/json" -H "mcp-session-id: ${MCP_SESSION_ID}" -d '{ "jsonrpc": "2.0", "method": "notifications/initialized"}' http://localhost:8888/mcp/



curl -i -X POST -H "Accept: application/json, text/event-stream" \
     -H "Content-Type: application/json" \
     -H "mcp-session-id: ${MCP_SESSION_ID}" \
     -d @- http://localhost:8888/mcp/ <<EOF
{
    "jsonrpc": "2.0",
    "id": "1",
    "method": "tools/call",
    "params": {
        "name": "add",
        "arguments": {
            "a": 10,
            "b": 100
        }
    }
}   
EOF


"""


from fastmcp import FastMCP
from fastmcp.server.middleware.logging import StructuredLoggingMiddleware
from fastmcp.server.dependencies import get_http_request
from starlette.requests import Request

    
mcp = FastMCP("Demo 🚀")
mcp.add_middleware(StructuredLoggingMiddleware(include_payloads=True))


@mcp.tool
def add(a: int, b: int) -> int:
    """Add two numbers"""
    return a + b


if __name__ == "__main__":
    # mcp.run()
    mcp.run(transport="http", host="0.0.0.0", port=8888, log_level="TRACE")


