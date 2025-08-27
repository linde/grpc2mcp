package proxy

import (
	"context"
	"errors"
	"fmt"
	"grpc2mcp/internal/mcpconst"
	"grpc2mcp/pb"

	"log"
	"net"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func RunServerAsync(handler http.Handler) (net.Listener, context.CancelFunc, error) {

	noopCancelFunc := func() {}
	listener, err := net.Listen("tcp", ":0") // use any open port
	if err != nil {
		return nil, noopCancelFunc, fmt.Errorf("net.Listen() failed: %w", err)
	}

	server := &http.Server{
		Handler: handler,
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		server.Serve(listener)
	}()

	// This listens for the ctx cancel() func, then triggers graceful shutdown
	go func() {
		<-ctx.Done()
		log.Printf("RunServerAsync's ctx cancel()'ed")

		// setup a context to limit graceful shutdown to at most 5s
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	return listener, cancel, nil
}

func doMcpInitialize(ctx context.Context, mcpGrpcClient pb.ModelContextProtocolClient) (context.Context, error) {

	var sessionHeader metadata.MD
	_, err := mcpGrpcClient.Initialize(ctx, &pb.InitializeRequest{}, grpc.Header(&sessionHeader))
	if err != nil {
		return nil, fmt.Errorf("error making Initialize grpc call: %w", err)
	}

	mcpSessionId := sessionHeader.Get(mcpconst.MCP_SESSION_ID_HEADER)
	if len(mcpSessionId) < 1 {
		errStr := fmt.Sprintf("did not receive mcp session id: %s", mcpconst.MCP_SESSION_ID_HEADER)
		return nil, errors.New(errStr)
	}
	clientCtx := metadata.NewOutgoingContext(context.Background(), sessionHeader)

	return clientCtx, nil

}
