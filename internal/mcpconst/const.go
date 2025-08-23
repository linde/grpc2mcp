package mcpconst

import "net/http"

// TODO make this a comparable type
var MCP_SESSION_ID_HEADER = http.CanonicalHeaderKey("mcp-session-id")

// Method is a typed string for JSON-RPC method names.
type JsonRpcMethod string

// Defines the standard JSON-RPC methods for MCP.
const (
	Initialize               JsonRpcMethod = "initialize"
	NotificationsInitialized JsonRpcMethod = "notifications/initialized"
	ToolsCall                JsonRpcMethod = "tools/call"
	Ping                     JsonRpcMethod = "ping"
)
