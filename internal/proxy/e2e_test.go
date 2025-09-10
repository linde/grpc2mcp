package proxy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestE2e(t *testing.T) {

	mcpGrpcClient, closeFunc, err := SetupAsyncMcpAndProxy(t.Name())
	require.NoError(t, err)
	defer closeFunc()

	ctx := context.Background()
	err = doGrpcProxyTests(ctx, mcpGrpcClient)
	require.NoError(t, err)

}

func TestAllTools(t *testing.T) {

	mcpGrpcClient, closeFunc, err := SetupAsyncMcpAndProxy(t.Name())
	require.NoError(t, err)
	defer closeFunc()

	ctx := context.Background()
	err = doGrpcProxyToolTests(ctx, mcpGrpcClient)
	require.NoError(t, err)

}
