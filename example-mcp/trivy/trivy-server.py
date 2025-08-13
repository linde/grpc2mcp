
import argparse
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


@mcp.tool
def mult(a: int, b: int) -> int:
    """Multiply two numbers"""
    return a * b

@mcp.tool
def lower(s: str) -> str:
    """lower case a string"""
    return s.lower()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="FastMCP Trivy Server")
    parser.add_argument("--port", type=int, default=8888, help="Port to run the server on")
    args = parser.parse_args()

    mcp.run(transport="http", host="0.0.0.0", port=args.port, log_level="TRACE")


