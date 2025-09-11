package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"grpc2mcp/internal/jsonrpc"
	"grpc2mcp/internal/mcpconst"
	"grpc2mcp/pb"
	mcp "grpc2mcp/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Server is the gRPC server that implements the ModelContextProtocolServer interface.
type Server struct {
	mcpUrl     string
	httpClient http.Client
}

func NewServer(mcpUrl string) (*Server, error) {
	return &Server{
		mcpUrl: mcpUrl,
	}, nil
}

// Start starts the gRPC server in its own goroutine. returns a func to shut it down.
func (s *Server) StartAsync(port int) (*net.TCPAddr, context.CancelFunc, error) {

	noopCancelFunc := func() {}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, noopCancelFunc, err
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(sessionInterceptor),
	)
	mcp.RegisterModelContextProtocolServer(grpcServer, s)
	reflection.Register(grpcServer)
	go func() {
		err := grpcServer.Serve(lis)
		if err != nil {
			log.Printf("grpcServer.Serve error: %v", err)
		}
	}()

	tcpAddr, _ := lis.Addr().(*net.TCPAddr)
	return tcpAddr, grpcServer.GracefulStop, nil
}

// StartProxyToListenerAsync starts the gRPC server in its own goroutine. returns a func to shut it down.
func (s *Server) StartProxyToListenerAsync(lis net.Listener) (func(), error) {

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(sessionInterceptor),
	)
	mcp.RegisterModelContextProtocolServer(grpcServer, s)
	reflection.Register(grpcServer)
	go func() {
		err := grpcServer.Serve(lis)
		if err != nil {
			log.Printf("grpcServer.Serve error: %v", err)
		}
	}()

	return grpcServer.GracefulStop, nil
}

// sessionInterceptor is a gRPC unary interceptor that checks for the MCP_SESSION_ID_HEADER header.
func sessionInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {

	// first, check for Authorization header because they're intended for
	// the downstream MCP server (ie as opposed to our service which would get a
	// dedicated interceptor. So, we want to make them available in the ctx

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "missing metadata")
	}

	authorizationHeader := md.Get(mcpconst.AuthorizationHeader)
	if len(authorizationHeader) > 0 {
		// what TODO if there are more than one?
		// looks like we have at least one header, use the first.

		ctx = context.WithValue(ctx, mcpconst.AuthorizationHeader, authorizationHeader[0]) //nolint:go-staticcheck // SA1029 ignore this!
	}

	// Next step is to check/process the session id which follows for all but
	// the Initialize method. Skip it bc that's where the session comes from
	if strings.HasSuffix(info.FullMethod, "Initialize") {
		return handler(ctx, req)
	}

	sessionID := md.Get(mcpconst.MCP_SESSION_ID_HEADER)
	if len(sessionID) == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "missing header: %s", mcpconst.MCP_SESSION_ID_HEADER)
	}

	// The session ID is valid, so we can proceed with the request.
	// Before we do, let's add the session ID to the context so that the
	// RPC handlers can access it.
	ctx = context.WithValue(ctx, mcpconst.MCP_SESSION_ID_HEADER, sessionID[0])

	return handler(ctx, req)
}

func initHeadersFromContext(ctx context.Context) map[string]string {

	headersFromContext := map[string]string{}

	CANDIDATE_HEADERS := []string{mcpconst.MCP_SESSION_ID_HEADER, mcpconst.AuthorizationHeader}

	for _, candidateHeader := range CANDIDATE_HEADERS {
		headerVal := ctx.Value(candidateHeader)
		if headerValStr, ok := headerVal.(string); ok && headerValStr != "" {
			// log.Printf("passing through header: %s:%s", candidateHeader, headerValStr)
			headersFromContext[candidateHeader] = headerValStr
		}
	}

	return headersFromContext
}

// initialize sends the 'initialize' request and synchronously parses the SSE response to get a session ID.
func (s *Server) doInitializeJsonRpc(ctx context.Context, req *mcp.InitializeRequest) (string, error) {
	log.Println("Initializing MCP session...")

	additionalHeaders := initHeadersFromContext(ctx)
	httpReq, err := jsonrpc.NewJSONRPCRequest(ctx, s.mcpUrl, mcpconst.Initialize, req, additionalHeaders, http.NewRequestWithContext)
	if err != nil {
		return "", status.Errorf(codes.Internal, "failed 'initialize' jsonrpc request: %v", err)
	}

	_, httpResp, err := jsonrpc.DoRequest(ctx, &s.httpClient, httpReq)
	if err != nil {
		return "", err // DoRequest already wraps the error.
	}

	mcpSessionId, ok := httpResp.Header[mcpconst.MCP_SESSION_ID_HEADER]
	if !ok || len(mcpSessionId) < 1 {
		return "", fmt.Errorf("did not find MCP Session ID header: %s", mcpconst.MCP_SESSION_ID_HEADER)
	}

	return mcpSessionId[0], nil
}

// follows up initialize() with an initialized() (notice the past tense) call to confirm a session
func (s *Server) doInitializedJsonRpc(ctx context.Context) error {
	log.Println("acking MCP session initializaton ...")

	additionalHeaders := initHeadersFromContext(ctx)
	httpReq, err := jsonrpc.NewJSONRPCRequest(ctx, s.mcpUrl, mcpconst.NotificationsInitialized, nil, additionalHeaders, http.NewRequestWithContext)
	if err != nil {
		return status.Errorf(codes.Internal, "failed 'initialized' http request: %v", err)
	}

	_, _, err = jsonrpc.DoRequest(ctx, &s.httpClient, httpReq)
	return err
}

// Initialize implements the Initialize RPC.
func (s *Server) Initialize(ctx context.Context, req *mcp.InitializeRequest) (*mcp.InitializeResult, error) {
	log.Println("Initialize called...")

	sessionID, err := s.doInitializeJsonRpc(ctx, req)
	if err != nil || sessionID == "" {
		return nil, status.Errorf(codes.Internal, "failed to initialize MCP session: %v", err)
	}

	// tuck the sessionId into the ctx for the subsequent Initialized ack call
	ctx = context.WithValue(ctx, mcpconst.MCP_SESSION_ID_HEADER, sessionID)

	if err := s.doInitializedJsonRpc(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to ack MCP session initialization: %v", err)
	}

	// Set the session ID in the response header
	if err := grpc.SetHeader(ctx, metadata.Pairs(mcpconst.MCP_SESSION_ID_HEADER, sessionID)); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to set session ID in header: %v", err)
	}

	log.Printf("Initialize and Initiailized complete for: %s", sessionID)

	// TODO: This should be a real response from the server
	return &mcp.InitializeResult{}, nil
}

// CallMethod implements the CallMethod RPC.
func (s *Server) CallMethod(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return s.doCallMethodRpc(ctx, req)
}

// doCallMethodRpc handles the specific logic for unmarshaling the polymorphic
// content in a CallToolResult.
func (s *Server) doCallMethodRpc(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	additionalHeaders := initHeadersFromContext(ctx)
	httpReq, err := jsonrpc.NewJSONRPCRequest(ctx, s.mcpUrl, mcpconst.ToolsCall, req, additionalHeaders, http.NewRequestWithContext)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create http request for %s: %v", mcpconst.ToolsCall, err)
	}

	resp, _, err := jsonrpc.DoRequest(ctx, &s.httpClient, httpReq)
	if err != nil {
		return nil, err // DoRequest already wraps the error.
	}

	if resp == nil {
		return nil, status.Errorf(codes.Internal, "MCP server returned a nil response")
	}

	if resp.Error != nil {
		return nil, status.Errorf(codes.Aborted, "MCP server returned an error (code %d): %s",
			resp.Error.Code, resp.Error.Message)
	}

	if resp.Result == nil {
		return nil, status.Errorf(codes.Internal, "MCP server returned a nil result")
	}

	// Custom unmarshaling for CallToolResult
	var rawResult struct {
		Content           []json.RawMessage `json:"content"`
		StructuredContent json.RawMessage   `json:"structuredContent"`
		IsError           bool              `json:"isError"`
	}

	if err := json.Unmarshal(*resp.Result, &rawResult); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmarshal raw result from mcp server: %v", err)
	}

	finalResult := &mcp.CallToolResult{
		IsError: &rawResult.IsError,
	}

	for _, rawContent := range rawResult.Content {
		var typeProbe struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(rawContent, &typeProbe); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to probe content type: %v", err)
		}

		var contentBlock mcp.ContentBlock
		switch typeProbe.Type {
		case "text":
			var textContent mcp.TextContent
			if err := json.Unmarshal(rawContent, &textContent); err != nil {
				return nil, status.Errorf(codes.Internal, "failed to unmarshal TextContent: %v", err)
			}
			contentBlock.ContentType = &mcp.ContentBlock_Text{Text: &textContent}
		case "resource_link":
			var resource mcp.Resource
			if err := json.Unmarshal(rawContent, &resource); err != nil {
				return nil, status.Errorf(codes.Internal, "failed to unmarshal ResourceLink: %v", err)
			}
			resourceLink := pb.ResourceLink{Type: typeProbe.Type, Resource: &resource}
			contentBlock.ContentType = &mcp.ContentBlock_ResourceLink{ResourceLink: &resourceLink}
		// TODO: Add cases for ImageContent, AudioContent, etc. as needed
		default:
			log.Printf("unknown content type: %s", typeProbe.Type)
			continue
		}
		finalResult.Content = append(finalResult.Content, &contentBlock)
	}

	return finalResult, nil
}

// ListTools implements the ListTools RPC.
func (s *Server) ListTools(ctx context.Context, req *mcp.ListToolsRequest) (*mcp.ListToolsResult, error) {
	var listToolsResult mcp.ListToolsResult
	err := s.doRpcCall(ctx, req, "tools/list", &listToolsResult)
	return &listToolsResult, err
}

func (s *Server) Complete(ctx context.Context, req *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
	var result mcp.CompleteResult
	err := s.doRpcCall(ctx, req, "completion/complete", &result)
	return &result, err
}

func (s *Server) Ping(ctx context.Context, req *mcp.PingRequest) (*mcp.PingResult, error) {
	var result mcp.PingResult
	err := s.doRpcCall(ctx, req, mcpconst.Ping, &result)
	return &result, err
}

// ListPrompts implements the ListPrompts RPC.
func (s *Server) ListPrompts(ctx context.Context, req *mcp.ListPromptsRequest) (*mcp.ListPromptsResult, error) {
	var result mcp.ListPromptsResult
	err := s.doRpcCall(ctx, req, "prompts/list", &result)
	return &result, err
}

// GetPrompt implements the GetPrompt RPC.
func (s *Server) GetPrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	var result mcp.GetPromptResult
	err := s.doRpcCall(ctx, req, "prompts/get", &result)
	return &result, err
}

// ListResources implements the ListResources RPC.
func (s *Server) ListResources(ctx context.Context, req *mcp.ListResourcesRequest) (*mcp.ListResourcesResult, error) {
	var result mcp.ListResourcesResult
	err := s.doRpcCall(ctx, req, "resources/list", &result)
	return &result, err
}

// ListResourceTemplates implements the ListResourceTemplates RPC.
func (s *Server) ListResourceTemplates(ctx context.Context, req *mcp.ListResourceTemplatesRequest) (*mcp.ListResourceTemplatesResult, error) {
	var result mcp.ListResourceTemplatesResult
	err := s.doRpcCall(ctx, req, "resources/templates/list", &result)
	return &result, err
}

// This is the heart of doing a session jsonrpc call and unpacking, then deserializing the result.
func (s *Server) doRpcCall(ctx context.Context, req protoreflect.ProtoMessage,
	jsonRpcMethod mcpconst.JsonRpcMethod, rpcResultPtr any) error {

	additionalHeaders := initHeadersFromContext(ctx)
	httpReq, err := jsonrpc.NewJSONRPCRequest(ctx, s.mcpUrl, jsonRpcMethod, req, additionalHeaders, http.NewRequestWithContext)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create http request for %s: %v", jsonRpcMethod, err)
	}

	resp, _, err := jsonrpc.DoRequest(ctx, &s.httpClient, httpReq)
	if err != nil {
		return err // DoRequest already wraps the error.
	}

	if resp == nil {
		// This can happen for notifications that succeed with no content.
		return nil
	}

	if resp.Error != nil {
		return status.Errorf(codes.Aborted, "MCP server returned an error (code %d): %s",
			resp.Error.Code, resp.Error.Message)
	}

	if resp.Result == nil {
		return status.Errorf(codes.Internal, "MCP server returned a nil result")
	}

	if err := json.Unmarshal(*resp.Result, rpcResultPtr); err != nil {
		return status.Errorf(codes.Internal, "failed to unmarshal result from mcp server: %v", err)
	}

	return nil
}
