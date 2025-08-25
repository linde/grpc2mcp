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
	"grpc2mcp/internal/mcpconst"
	mcp "grpc2mcp/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type mcpSession struct {
	sessionID string
}

// Server is the gRPC server that implements the ModelContextProtocolServer interface.
type Server struct {
	mcpUrl     string
	httpClient http.Client
}

func (s *Server) getMcpUrl() string {
	return s.mcpUrl
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
		ctx = context.WithValue(ctx, mcpconst.AuthorizationHeader, authorizationHeader[0])
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
	httpReq, err := jsonrpc.NewJSONRPCRequest(s.mcpUrl, "initialize", req, additionalHeaders, http.NewRequest)

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
	httpReq, err := jsonrpc.NewJSONRPCRequest(s.mcpUrl, "notifications/initialized", nil, additionalHeaders, http.NewRequest)

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
	err := s.doRpcCall(ctx, req, mcpconst.Ping, &result)

	return &result, err
}

// This is the heart of doing a session jsonrpc call and unpacking, then deserializing the result.
func (s *Server) doRpcCall(ctx context.Context, req protoreflect.ProtoMessage,
	jsonRpcMethod mcpconst.JsonRpcMethod, rpcResultPtr any) error {

	additionalHeaders := initHeadersFromContext(ctx)
	jsonRpcResponseParts, err := jsonrpc.GetJSONRPCRequestResponse(ctx, s.mcpUrl, jsonRpcMethod, req, additionalHeaders)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to parse mcp server response: %v", err)
	}

	// there seem to be two ways jsonrpc content comes in the response body, either:
	// * in newline delimited key/value pairs where what we want is prefixed by "data:"
	// * or as in the github mcp server, just the body itself in one part.

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
