package proxy

import (
	"context"
	"grpc2mcp/internal/examplemcp"
	"grpc2mcp/pb"
	"log"
	"net"
	"net/http/httptest"
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

	ts := httptest.NewServer(handler)
	defer ts.Close()

	log.Printf("mcp handler listening on: %s", ts.URL)
	s, err := NewServer(ts.URL)
	assert.NoError(err)

	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)
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

	err = doGrpcProxyTests(context.Background(), mcpGrpcClient)
	assert.NoError(err)

	err = doGrpcProxyToolTests(context.Background(), mcpGrpcClient)
	assert.NoError(err)

	err = doGrpcProxyPromptTests(context.Background(), mcpGrpcClient)
	assert.NoError(err)

}
