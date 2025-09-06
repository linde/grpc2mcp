package examplemcp

import (
	"context"
	"grpc2mcp/internal/jsonrpc"
	"grpc2mcp/internal/mcpconst"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrivyServerTools(t *testing.T) {
	assert := assert.New(t)

	exampleMcpServerUri := "/mcp"
	handler := RunExampleMcpServer(t.Name(), exampleMcpServerUri)
	ts := httptest.NewServer(handler)
	defer ts.Close()
	assert.NotNil(ts)

	log.Printf("mcp handler listening on: %s", ts.URL)

	ctx := context.Background()

	// 1. Initialize

	// TODO consider moving this into jsonrpc maybe?
	initParams := map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": t.Name(), "version": "1.0"},
	}

	testNewRequesterFunc := func(ctx context.Context, method string, url string, body io.Reader) (*http.Request, error) {
		return httptest.NewRequest(method, url, body), nil
	}

	noAddlHeaders := map[string]string{} // no headers
	initReq, err := jsonrpc.NewJSONRPCRequest(ctx, exampleMcpServerUri, mcpconst.Initialize,
		initParams, noAddlHeaders, testNewRequesterFunc)
	assert.NoError(err)

	initRR := httptest.NewRecorder()
	handler.ServeHTTP(initRR, initReq)
	assert.Equal(http.StatusOK, initRR.Code)

	sessionID := initRR.Header().Get(mcpconst.MCP_SESSION_ID_HEADER)
	assert.NotEmpty(sessionID)
	log.Printf("initialze session: %s", sessionID)

	// 2. Initialized
	sessionIdHeader := map[string]string{mcpconst.MCP_SESSION_ID_HEADER: sessionID}
	initializedReq, err := jsonrpc.NewJSONRPCRequest(ctx, exampleMcpServerUri,
		mcpconst.NotificationsInitialized, nil, sessionIdHeader, testNewRequesterFunc)
	assert.NoError(err)

	initializedRR := httptest.NewRecorder()
	handler.ServeHTTP(initializedRR, initializedReq)
	assert.Equal(http.StatusAccepted, initializedRR.Code)
	log.Printf("initialized: %s", sessionID)

	// 2. Ping
	pingReq, err := jsonrpc.NewJSONRPCRequest(ctx, exampleMcpServerUri,
		mcpconst.Ping, nil, sessionIdHeader, testNewRequesterFunc)
	assert.NoError(err)

	pingRR := httptest.NewRecorder()
	handler.ServeHTTP(pingRR, pingReq)
	assert.Equal(http.StatusOK, pingRR.Code)
	log.Printf("ping status: %d", pingRR.Code)

	// leave the prompts and tools to the e2e tests
}
