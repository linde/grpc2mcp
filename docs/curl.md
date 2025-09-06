

# Curl Samples

The follow show raw `curl` examples which the grpc proxy is proxying. To try them,
run a server using either of the MCP examples in ([example-mcp](../example-mcp/README.md))
or point to your favorite other MCP server.


```bash
export MCP_URL=http://localhost:8888/mcp

# start initialization
curl -i -X POST -H "Accept: application/json, text/event-stream" \
    -H "Content-Type: application/json" \
     -d @- ${MCP_URL} <<EOF
{ 
    "jsonrpc": "2.0", 
    "id": "1", 
    "method": "initialize", 
    "params": { 
        "protocolVersion": "2025-06-18", 
        "capabilities": {}, 
        "clientInfo": { "name": "curl", "version": "1.0" } 
    }
}
EOF

# get the mcp-session-id header and export it as MCP_SESSION_ID
export MCP_SESSION_ID=[get it from above curl call response header]

# send confirmation the session is initialated, should return 202 Accepted
curl -i -X POST -H "Accept: application/json, text/event-stream" \
    -H "Content-Type: application/json" \
    -H "mcp-session-id: ${MCP_SESSION_ID}" \
    -d '{ "jsonrpc": "2.0", "method": "notifications/initialized"}' ${MCP_URL}


# then, list what's available
curl -i -X POST -H "Accept: application/json, text/event-stream" \
    -H "Content-Type: application/json" \
    -H "mcp-session-id: ${MCP_SESSION_ID}" \
    -d '{ "jsonrpc": "2.0", "id": 1, "method": "tools/list"}' ${MCP_URL}

# then make tool calls!

curl -i -X POST -H "Accept: application/json, text/event-stream" \
     -H "Content-Type: application/json" \
     -H "mcp-session-id: ${MCP_SESSION_ID}" \
     -d @- ${MCP_URL} <<EOF
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

# send a completion request
curl -i -X POST -H "Accept: application/json, text/event-stream" \
     -H "Content-Type: application/json" \
     -H "mcp-session-id: ${MCP_SESSION_ID}" \
     ${MCP_URL} -d @- <<EOF
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "completion/complete",
  "params": {
    "ref": {
      "type": "ref/prompt",
      "name": "code_review"
    },
    "argument": {
      "name": "language",
      "value": "py"
    }
  }
}
EOF


# send a ping
curl -i -X POST -H "Accept: application/json, text/event-stream" \
     -H "Content-Type: application/json" \
     -H "mcp-session-id: ${MCP_SESSION_ID}" \
     -d @- ${MCP_URL} <<EOF
{
  "jsonrpc": "2.0",
  "id": "123",
  "method": "ping"
}
EOF

```