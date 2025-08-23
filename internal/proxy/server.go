package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"

	"grpc2mcp/internal/jsonrpc"
	mcp "grpc2mcp/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var MCP_SESSION_ID_HEADER = http.CanonicalHeaderKey("mcp-session-id")

// Server is the gRPC server that implements the ModelContextProtocolServer interface.
type Server struct {
	mcp.UnimplementedModelContextProtocolServer
	mcpHost    string
	mcpPort    int
	mcpUri     string
	httpClient *http.Client
}

// NewServer creates a new Server and initializes a session with the downstream MCP server.
func NewServer(mcpHost string, mcpPort int, mcpUri string) (*Server, error) {
	s := &Server{
		mcpHost:    mcpHost,
		mcpPort:    mcpPort,
		mcpUri:     mcpUri,
		httpClient: &http.Client{}, // this seems needed by grpc,
	}

	return s, nil
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

// sessionInterceptor is a gRPC unary interceptor that checks for the mcp-session-id header.
func sessionInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	// bypass the interceptor for the Initialize method
	if strings.HasSuffix(info.FullMethod, "Initialize") {
		return handler(ctx, req)
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "missing metadata")
	}

	sessionID := md.Get(MCP_SESSION_ID_HEADER)
	if len(sessionID) == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "missing mcp-session-id header")
	}

	// The session ID is valid, so we can proceed with the request.
	// Before we do, let's add the session ID to the context so that the
	// RPC handlers can access it.
	ctx = context.WithValue(ctx, MCP_SESSION_ID_HEADER, sessionID[0])

	return handler(ctx, req)
}

// initialize sends the 'initialize' request and synchronously parses the SSE response to get a session ID.
func (s *Server) doInitializeJsonRpc(ctx context.Context, req *mcp.InitializeRequest) (string, error) {

	log.Println("Initializing MCP session...")

	httpReq, err := jsonrpc.NewJSONRPCRequest(ctx, s.mcpHost, s.mcpPort, s.mcpUri, "initialize", req, nil)
	if err != nil {
		return "", status.Errorf(codes.Internal, "failed 'initialize' jsonrpc request: %v", err)
	}

	httpResp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send initialize request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return "", fmt.Errorf("initialize request failed with status %d: %s", httpResp.StatusCode, string(body))
	}

	mcpSessionId, ok := httpResp.Header[MCP_SESSION_ID_HEADER]
	if !ok || len(mcpSessionId) < 1 {
		return "", fmt.Errorf("did not find MCP Session ID header: %s", MCP_SESSION_ID_HEADER)
	}

	return mcpSessionId[0], nil
}

// follows up initialize() with an initialized() (notice the past tense) call to confirm a session
func (s *Server) doInitializedJsonRpc(ctx context.Context, sessionID string) error {

	log.Println("acking MCP session initializaton ...")

	sessionHeader := map[string]string{MCP_SESSION_ID_HEADER: sessionID}
	httpReq, err := jsonrpc.NewJSONRPCRequest(ctx, s.mcpHost, s.mcpPort, s.mcpUri, "notifications/initialized", nil, sessionHeader)
	if err != nil {
		return status.Errorf(codes.Internal, "failed 'initialized' http request: %v", err)
	}

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

// Initialize implements the Initialize RPC.
func (s *Server) Initialize(ctx context.Context, req *mcp.InitializeRequest) (*mcp.InitializeResult, error) {
	log.Println("Initialize called...")

	sessionID, err := s.doInitializeJsonRpc(ctx, req)
	if err != nil || sessionID == "" {
		return nil, status.Errorf(codes.Internal, "failed to initialize MCP session: %v", err)
	}
	if err := s.doInitializedJsonRpc(ctx, sessionID); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to ack MCP session initialization: %v", err)
	}

	// Set the session ID in the response header
	if err := grpc.SetHeader(ctx, metadata.Pairs(MCP_SESSION_ID_HEADER, sessionID)); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to set session ID in header: %v", err)
	}

	log.Printf("Initialize and Initiailized complete for: %s", sessionID)

	// TODO: This should be a real response from the server
	return &mcp.InitializeResult{}, nil
}

// CallMethod implements the CallMethod RPC.
func (s *Server) CallMethod(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// log.Printf("CallMethod requested for method '%s': %v", req.Name, req)

	var result mcp.CallToolResult
	err := s.doRpcCall(ctx, req, "tools/call", &result)

	return &result, err
}

// ListTools implements the ListTools RPC.
func (s *Server) ListTools(ctx context.Context, req *mcp.ListToolsRequest) (*mcp.ListToolsResult, error) {
	log.Printf("ListTools requested: %v", req)

	var listToolsResult mcp.ListToolsResult

	err := s.doRpcCall(ctx, req, "tools/list", &listToolsResult)
	return &listToolsResult, err

}

func (s *Server) Complete(ctx context.Context, req *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
	log.Printf("Complete requested: %v", req)

	var result mcp.CompleteResult
	err := s.doRpcCall(ctx, req, "completion/complete", &result)

	return &result, err
}

func (s *Server) Ping(ctx context.Context, req *mcp.PingRequest) (*mcp.PingResult, error) {
	log.Printf("Ping requested: %v", req)

	var result mcp.PingResult
	err := s.doRpcCall(ctx, req, "ping", &result)

	return &result, err
}

// This is the heart of doing a session jsonrpc call and unpacking, then deserializing the result.
func (s *Server) doRpcCall(ctx context.Context, req protoreflect.ProtoMessage, method string, rpcResultPtr any) error {

	sessionID, ok := ctx.Value(MCP_SESSION_ID_HEADER).(string)
	if !ok {
		return status.Errorf(codes.Internal, "could not get session id (%s) from context", MCP_SESSION_ID_HEADER)
	}

	headers := map[string]string{MCP_SESSION_ID_HEADER: sessionID}
	jsonRpcResponseParts, err := jsonrpc.GetJSONRPCRequestResponse(ctx, s.mcpHost, s.mcpPort, s.mcpUri, method, req, headers)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to parse mcp server response: %v", err)
	}

	var jsonRPCResp jsonrpc.JSONRPCResponse
	if err := json.Unmarshal([]byte(jsonRpcResponseParts["data"]), &jsonRPCResp); err != nil {
		return status.Errorf(codes.Internal, "failed to unmarshal mcp server response: %v", err)
	}

	if jsonRPCResp.Error != nil {
		return status.Errorf(codes.Aborted, "MCP server returned an error (code %d): %s",
			jsonRPCResp.Error.Code, jsonRPCResp.Error.Message)
	}

	if err := json.Unmarshal(jsonRPCResp.Result, rpcResultPtr); err != nil {
		return status.Errorf(codes.Internal, "failed to unmarshal result from mcp server: %v", err)
	}

	return nil
}
