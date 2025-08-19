package proxy

import (
	"context"
	"grpc2mcp/pb"
	"testing"

	asserts "github.com/stretchr/testify/assert"
)

func TestE2e(t *testing.T) {

	assert := asserts.New(t)

	conn, closeFunc, err := SetupMcpAndProxyAsync(t.Name())
	assert.NoError(err)
	defer conn.Close()
	defer closeFunc()

	mcpGrpcClient := pb.NewModelContextProtocolClient(conn)
	ctx := context.Background()

	err = doGrpcProxyTests(ctx, mcpGrpcClient)
	assert.NoError(err)

}

func TestAllTools(t *testing.T) {
	assert := asserts.New(t)

	conn, closeFunc, err := SetupMcpAndProxyAsync(t.Name())
	assert.NoError(err)
	defer conn.Close()
	defer closeFunc()

	mcpGrpcClient := pb.NewModelContextProtocolClient(conn)
	ctx := context.Background()

	err = doGrpcProxyToolTests(ctx, mcpGrpcClient)
	assert.NoError(err)

}
