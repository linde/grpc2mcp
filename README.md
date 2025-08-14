# gRPC to MCP Proxy

This project implements a proxy server that translates gRPC requests into JSON-RPC requests for a server that conforms to the Model Context Protocol (MCP). This allows gRPC-based clients to communicate with MCP-compatible servers seamlessly.

The proxy handles the gRPC service defined in `proto/mcp.proto` and forwards the requests to a configured MCP endpoint.

## Running the Example MCP Server

An example MCP server is provided in this repository built using [fastmcp](https://gofastmcp.com/). To run it, you first need to activate the Python virtual environment and then run the server script.

```bash
# Activate the virtual environment
source .venv/bin/activate

# Run the trivy server
python example-mcp/trivy/trivy-server.py
```

The example MCP server will run on `localhost:8888` by default. You can specify 
other ports via `--port`.

## Running the Proxy

Once the example MCP server is running, you can run the proxy in a separate terminal. The proxy will start and listen for incoming gRPC connections on the specified port.

```bash
go run main.go proxy [flags]
```

### Flags

*   `--port`: The port for the gRPC proxy to listen on (default: `8080`).
*   `--mcp-host`: The hostname of the downstream MCP server (default: `localhost`).
*   `--mcp-port`: The port of the downstream MCP server (default: `8888`).
*   `--mcp-uri`: The URI path for the MCP server endpoint (default: `/mcp/`).

### Example

To start the proxy and have it connect to the example MCP server running on `localhost:8888`, you would run:

```bash
go run main.go proxy --port 8080 --mcp-host localhost --mcp-port 8888
```

### Example Usage with `grpcurl`

Once the proxy is running, you can use tools like `grpcurl` to try things out. 
The following call examples with `add`, `mult` and `lower` tools with appropriate 
arguments after first initiailizing an MCP session:



```bash

# first initialize to see the whole response
 grpcurl -v -plaintext   localhost:8080  mcp.ModelContextProtocol/Initialize

# try it again with grep to capture the value of the "mcp-session-id" header line
MCP_SESSION_HEADER=$(grpcurl -v -plaintext   localhost:8080    mcp.ModelContextProtocol/Initialize | grep mcp-session-id)

# now use that header with the session id to make any other calls
grpcurl -H "${MCP_SESSION_HEADER}" -plaintext \
    -d '{"method": "tools/call", "params": {"name": "add", "arguments": {"a": 20, "b": 1}}}' \
    localhost:8080    mcp.ModelContextProtocol/CallMethod

grpcurl -H "${MCP_SESSION_HEADER}" -plaintext \
    -d '{"method": "tools/call", "params": {"name": "mult", "arguments": {"a": 20, "b": 1}}}' \
    localhost:8080    mcp.ModelContextProtocol/CallMethod

grpcurl -H "${MCP_SESSION_HEADER}" -plaintext \
    -d '{"method": "tools/call", "params": {"name": "lower", "arguments": {"s": "thisIsMixedCase"}}}' \
    localhost:8080 mcp.ModelContextProtocol/CallMethod

