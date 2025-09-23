package mcpconst

// TODO make this a comparable type
var MCP_SESSION_ID_HEADER = "mcp-session-id"
var AuthorizationHeader = "authorization"

// Method is a typed string for JSON-RPC method names.
type JsonRpcMethod string

// Defines the standard JSON-RPC methods for MCP.
const (
	Initialize               JsonRpcMethod = "initialize"
	NotificationsInitialized JsonRpcMethod = "notifications/initialized"
	ToolsCall                JsonRpcMethod = "tools/call"
	Ping                     JsonRpcMethod = "ping"
)
