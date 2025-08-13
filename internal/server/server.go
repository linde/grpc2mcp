package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"

	mcp "grpc2mcp/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

var MCP_SESSION_ID_HEADER = http.CanonicalHeaderKey("mcp-session-id")

// Server is the gRPC server that implements the ModelContextProtocolServer interface.
type Server struct {
	mcp.UnimplementedModelContextProtocolServer
	mcpHost    string
	mcpPort    int
	mcpUri     string
	sessionID  string
	httpClient *http.Client
}

// NewServer creates a new Server and initializes a session with the downstream MCP server.
func NewServer(mcpHost string, mcpPort int, mcpUri string) (*Server, error) {
	s := &Server{
		mcpHost:    mcpHost,
		mcpPort:    mcpPort,
		mcpUri:     mcpUri,
		httpClient: &http.Client{},
	}

	if err := s.initialize(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize MCP session: %w", err)
	}
	if err := s.ackInitialized(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize MCP session: %w", err)
	}

	return s, nil
}

// initialize sends the 'initialize' request and synchronously parses the SSE response to get a session ID.
func (s *Server) initialize(ctx context.Context) error {

	log.Println("Initializing MCP session...")

	params := map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]string{},
		"clientInfo":      map[string]string{"name": "curl", "version": "1.0"},
	}

	httpReq, err := NewJSONRPCRequest(ctx, s.mcpHost, s.mcpPort, s.mcpUri, "initialize", params)
	if err != nil {
		return status.Errorf(codes.Internal, "failed 'initialize' jsonrpc request: %v", err)
	}

	httpResp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send initialize request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("initialize request failed with status %d: %s", httpResp.StatusCode, string(body))
	}

	mcpSessionId, ok := httpResp.Header[MCP_SESSION_ID_HEADER]
	if !ok || len(mcpSessionId) < 1 {
		return fmt.Errorf("did not find MCP Session ID header: %s", MCP_SESSION_ID_HEADER)
	}

	s.sessionID = mcpSessionId[0]

	return nil
}

// initialize sends the 'initialize' request and synchronously parses the SSE response to get a session ID.
func (s *Server) ackInitialized(ctx context.Context) error {

	log.Println("acking MCP session initializaton ...")

	httpReq, err := NewJSONRPCRequest(ctx, s.mcpHost, s.mcpPort, s.mcpUri, "notifications/initialized", nil)
	if err != nil {
		return status.Errorf(codes.Internal, "failed 'initialized' http request: %v", err)
	}
	httpReq.Header.Set(MCP_SESSION_ID_HEADER, s.sessionID)

	httpResp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed 'initialized' request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("failed 'initialized' with status %d: %s", httpResp.StatusCode, string(body))
	}

	return nil

}

// Start starts the gRPC server.
func (s *Server) Start(port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer()
	mcp.RegisterModelContextProtocolServer(grpcServer, s)
	reflection.Register(grpcServer)
	log.Printf("gRPC server listening on port %d", port)
	return grpcServer.Serve(lis)
}

// CallTool implements the CallTool RPC.
func (s *Server) CallTool(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("CallTool requested: %v", req)

	if s.sessionID == "" {
		return nil, status.Errorf(codes.FailedPrecondition, "mcp session not initialized")
	}

	var params map[string]any
	paramsBytes, err := json.Marshal(req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to marshal request to json: %v", err)
	}
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmarshal params to map: %v", err)
	}

	httpReq, err := NewJSONRPCRequest(ctx, s.mcpHost, s.mcpPort, s.mcpUri, "tools/call", params)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create http request: %v", err)
	}
	httpReq.Header.Set(MCP_SESSION_ID_HEADER, s.sessionID)

	httpResp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "failed to call mcp server: %v", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read mcp server response: %v", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, status.Errorf(codes.Unavailable, "mcp server returned non-200 status: %d", httpResp.StatusCode)
	}

	jsonRpcResponseParts, err := parseJsonRpcResponseBody(respBody)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to parse mcp server response: %v", err)
	}

	// log.Printf("Parsed jsonRpcResponseParts: %v", jsonRpcResponseParts)

	var jsonRPCResp JSONRPCResponse
	if err := json.Unmarshal([]byte(jsonRpcResponseParts["data"]), &jsonRPCResp); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmarshal mcp server response: %v", err)
	}

	if jsonRPCResp.Error != nil {
		return nil, status.Errorf(codes.Aborted, "mcp server returned an error: %s", jsonRPCResp.Error.Message)
	}

	var tempResult struct {
		Content           []json.RawMessage `json:"content"`
		StructuredContent json.RawMessage   `json:"structuredContent"`
		IsError           bool              `json:"isError"`
	}

	if err := json.Unmarshal(jsonRPCResp.Result, &tempResult); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmarshal result from mcp server: %v", err)
	}

	var callToolResult mcp.CallToolResult
	callToolResult.IsError = &tempResult.IsError
	if err := json.Unmarshal(tempResult.StructuredContent, &callToolResult.StructuredContent); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmarshal structuredContent: %v", err)
	}

	for _, rawContent := range tempResult.Content {
		var contentType struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(rawContent, &contentType); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to unmarshal content type: %v", err)
		}

		var contentBlock mcp.ContentBlock
		switch contentType.Type {
		case "text":
			var textContent mcp.TextContent
			if err := json.Unmarshal(rawContent, &textContent); err != nil {
				return nil, status.Errorf(codes.Internal, "failed to unmarshal text content: %v", err)
			}
			contentBlock.ContentType = &mcp.ContentBlock_Text{Text: &textContent}
		// TODO: Add cases for other content types as needed
		default:
			return nil, status.Errorf(codes.Internal, "unknown content type: %s", contentType.Type)
		}
		callToolResult.Content = append(callToolResult.Content, &contentBlock)
	}

	return &callToolResult, nil
}

// ListTools implements the ListTools RPC.
func (s *Server) ListTools(ctx context.Context, req *mcp.ListToolsRequest) (*mcp.ListToolsResult, error) {
	log.Printf("ListTools requested: %v", req)

	if s.sessionID == "" {
		return nil, status.Errorf(codes.FailedPrecondition, "mcp session not initialized")
	}

	var params map[string]any
	paramsBytes, err := json.Marshal(req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to marshal request to json: %v", err)
	}
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmarshal params to map: %v", err)
	}

	httpReq, err := NewJSONRPCRequest(ctx, s.mcpHost, s.mcpPort, s.mcpUri, "tools/list", params)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create http request: %v", err)
	}
	httpReq.Header.Set(MCP_SESSION_ID_HEADER, s.sessionID)

	httpResp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "failed to call mcp server: %v", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read mcp server response: %v", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, status.Errorf(codes.Unavailable, "mcp server returned non-200 status: %d", httpResp.StatusCode)
	}

	jsonRpcResponseParts, err := parseJsonRpcResponseBody(respBody)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to parse mcp server response: %v", err)
	}

	// log.Printf("Parsed jsonRpcResponseParts: %v", jsonRpcResponseParts)

	var jsonRPCResp JSONRPCResponse
	if err := json.Unmarshal([]byte(jsonRpcResponseParts["data"]), &jsonRPCResp); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmarshal mcp server response: %v", err)
	}

	if jsonRPCResp.Error != nil {
		return nil, status.Errorf(codes.Aborted, "mcp server returned an error: %s", jsonRPCResp.Error.Message)
	}

	var tempResult struct {
		Tools      []json.RawMessage `json:"tools"`
		NextCursor string            `json:"next_cursor"`
	}
	if err := json.Unmarshal(jsonRPCResp.Result, &tempResult); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmarshal result from mcp server: %v", err)
	}

	var listToolsResult mcp.ListToolsResult
	if tempResult.NextCursor != "" {
		listToolsResult.NextCursor = &tempResult.NextCursor
	}

	// This struct mirrors the JSON structure from the Python server
	type pythonTool struct {
		Name         string          `json:"name"`
		Description  string          `json:"description"`
		InputSchema  json.RawMessage `json:"inputSchema"`
		OutputSchema json.RawMessage `json:"outputSchema"`
		Annotations  json.RawMessage `json:"annotations"`
		Meta         json.RawMessage `json:"_meta"`
	}

	for _, rawTool := range tempResult.Tools {
		var pt pythonTool
		if err := json.Unmarshal(rawTool, &pt); err != nil {
			log.Printf("Error unmarshaling tool: %s", string(rawTool))
			return nil, status.Errorf(codes.Internal, "failed to unmarshal tool into pythonTool: %v", err)
		}

		// Manually construct the mcp.Tool
		tool := &mcp.Tool{
			Metadata: &mcp.BaseMetadata{
				Name: pt.Name,
			},
			Description: &pt.Description,
		}

		if pt.InputSchema != nil {
			if err := json.Unmarshal(pt.InputSchema, &tool.InputSchema); err != nil {
				return nil, status.Errorf(codes.Internal, "failed to unmarshal input_schema: %v", err)
			}
		}
		if pt.OutputSchema != nil {
			if err := json.Unmarshal(pt.OutputSchema, &tool.OutputSchema); err != nil {
				return nil, status.Errorf(codes.Internal, "failed to unmarshal output_schema: %v", err)
			}
		}
		if pt.Annotations != nil {
			if err := json.Unmarshal(pt.Annotations, &tool.Annotations); err != nil {
				return nil, status.Errorf(codes.Internal, "failed to unmarshal annotations: %v", err)
			}
		}
		if pt.Meta != nil {
			if err := json.Unmarshal(pt.Meta, &tool.XMeta); err != nil {
				return nil, status.Errorf(codes.Internal, "failed to unmarshal _meta: %v", err)
			}
		}

		listToolsResult.Tools = append(listToolsResult.Tools, tool)
	}

	return &listToolsResult, nil
}
