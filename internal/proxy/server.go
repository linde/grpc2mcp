package proxy

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"

	mcp "grpc2mcp/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
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
		grpc.UnaryInterceptor(unarySessionInterceptor),
		grpc.StreamInterceptor(streamSessionInterceptor),
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
		grpc.UnaryInterceptor(unarySessionInterceptor),
		grpc.StreamInterceptor(streamSessionInterceptor),
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
