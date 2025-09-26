package proxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestE2e(t *testing.T) {

	mcpGrpcClient, closeFunc, err := SetupAsyncMcpAndProxy(t.Name())
	require.NoError(t, err)
	defer closeFunc()

	doGrpcProxyTests(t, mcpGrpcClient)

}

func TestAllTools(t *testing.T) {

	mcpGrpcClient, closeFunc, err := SetupAsyncMcpAndProxy(t.Name())
	require.NoError(t, err)
	defer closeFunc()

	doMcpClientTests(t, mcpGrpcClient)

}
