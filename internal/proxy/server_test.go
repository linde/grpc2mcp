package proxy

import (
	"context"
	"grpc2mcp/internal/examplemcp"
	"grpc2mcp/pb"
	"log"
	"net"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestBufconDirect(t *testing.T) {

	handler := examplemcp.RunExampleMcpServer(t.Name(), "/mcp")

	ts := httptest.NewServer(handler)
	defer ts.Close()

	log.Printf("mcp handler listening on: %s", ts.URL)
	s, err := NewServer(ts.URL)
	require.NoError(t, err)

	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)
	serverCancel, err := s.StartProxyToListenerAsync(lis)
	require.NoError(t, err)
	defer serverCancel()

	bufDialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(bufDialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	defer conn.Close()

	mcpGrpcClient := pb.NewModelContextProtocolClient(conn)

	err = doGrpcProxyTests(context.Background(), mcpGrpcClient)
	require.NoError(t, err)

	err = doGrpcProxyToolTests(context.Background(), mcpGrpcClient)
	require.NoError(t, err)

	err = doGrpcProxyPromptTests(context.Background(), mcpGrpcClient)
	require.NoError(t, err)

	err = doGrpcProxyResourceTests(context.Background(), mcpGrpcClient)
	require.NoError(t, err)

}
