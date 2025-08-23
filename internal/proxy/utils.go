package proxy

import (
	"context"
	"errors"
	"fmt"
	"grpc2mcp/internal/examplemcp"
	"grpc2mcp/internal/mcpconst"
	"grpc2mcp/pb"

	"log"
	"net"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

func SetupAsyncMcpAndProxy(mcpServerName string) (pb.ModelContextProtocolClient, func(), error) {

	accumulatedCancelFuncs := make([]func(), 3)
	aggregateCancelFunc := func() {
		log.Printf("shutting down mcp server and grpc proxy")
		for _, f := range accumulatedCancelFuncs {
			if f != nil {
				f()
			}
		}
	}

	handler := examplemcp.RunTrivyServer(mcpServerName)
	mcpListener, trivyServerCancelFunc, err := RunServerAsync(handler)
	if err != nil {
		return nil, aggregateCancelFunc, fmt.Errorf("error setting up proxy in SetupMcpAndProxy: %w", err)
	}
	accumulatedCancelFuncs = append(accumulatedCancelFuncs, trivyServerCancelFunc)

	// TODO figure out getting an IP from net.TCPAddr better, for now assume 0.0.0.0
	mcpTcpAddr, _ := mcpListener.Addr().(*net.TCPAddr)
	host, port := "0.0.0.0", mcpTcpAddr.Port
	log.Printf("mcp handler listening on: %s:%d", host, port)
	s, err := NewServer(host, port, "/")
	if err != nil {
		return nil, aggregateCancelFunc, fmt.Errorf("failed to create proxy server: %w", err)
	}

	proxyTcpAddr, proxyCancelFunc, err := s.StartAsync(0) // let system find open port
	accumulatedCancelFuncs = append(accumulatedCancelFuncs, proxyCancelFunc)
	if err != nil {
		return nil, aggregateCancelFunc, fmt.Errorf("failed to start proxy server: %w", err)
	}

	log.Printf("mcp grpc proxy listening on: %s", proxyTcpAddr)

	// put together a client and return it to the caller
	newClient, newClientErr := grpc.NewClient(proxyTcpAddr.String(), grpc.WithTransportCredentials(insecure.NewCredentials()))

	mcpGrpcClient := pb.NewModelContextProtocolClient(newClient)

	return mcpGrpcClient, aggregateCancelFunc, newClientErr

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
