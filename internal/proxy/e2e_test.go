package proxy

import (
	"context"
	"testing"

	asserts "github.com/stretchr/testify/assert"
)

func TestE2e(t *testing.T) {

	assert := asserts.New(t)

	mcpGrpcClient, closeFunc, err := SetupAsyncMcpAndProxy(t.Name())
	assert.NoError(err)
	defer closeFunc()

	ctx := context.Background()
	err = doGrpcProxyTests(ctx, mcpGrpcClient)
	assert.NoError(err)

}

func TestAllTools(t *testing.T) {
	assert := asserts.New(t)

	mcpGrpcClient, closeFunc, err := SetupAsyncMcpAndProxy(t.Name())
	assert.NoError(err)
	defer closeFunc()

	ctx := context.Background()
	err = doGrpcProxyToolTests(ctx, mcpGrpcClient)
	assert.NoError(err)

}
