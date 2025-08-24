package proxy

import (
	"context"
	"fmt"
	"grpc2mcp/internal/examplemcp"
	"grpc2mcp/pb"
	"log"
	"net"
	"testing"

	asserts "github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestBufconDirect(t *testing.T) {

	assert := asserts.New(t)
	assert.NotNil(assert)

	handler := examplemcp.RunTrivyServer(t.Name())
	mcpListener, trivyServerCancelFunc, err := RunServerAsync(handler)
	assert.NoError(err)
	defer trivyServerCancelFunc()

	// TODO figure out getting an IP from net.TCPAddr better, for now assume 0.0.0.0
	mcpTcpAddr, _ := mcpListener.Addr().(*net.TCPAddr)
	mcpUrl := fmt.Sprintf("http://0.0.0.0:%d/", mcpTcpAddr.Port)
	log.Printf("mcp handler listening on: %s", mcpUrl)

	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)

	// TODO create our own exampleMCP inst
	s, err := NewServer(mcpUrl)
	assert.NoError(err)

	serverCancel, err := s.StartProxyToListenerAsync(lis)
	assert.NoError(err)
	defer serverCancel()

	bufDialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(bufDialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		assert.NoError(err)
		return
	}
	defer conn.Close()

	mcpGrpcClient := pb.NewModelContextProtocolClient(conn)
	ctx := context.Background()

	err = doGrpcProxyTests(ctx, mcpGrpcClient)
	assert.NoError(err)

	err = doGrpcProxyToolTests(ctx, mcpGrpcClient)
	assert.NoError(err)

}
