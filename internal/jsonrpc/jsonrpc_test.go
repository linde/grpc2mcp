package jsonrpc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"grpc2mcp/internal/mcpconst"

	"github.com/sourcegraph/jsonrpc2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockNewHttpRequester is a mock implementation of NewHttpRequester for testing.
func mockNewHttpRequester(ctx context.Context, method string, url string, body io.Reader) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, method, url, body)
}

func TestNewJSONRPCRequest_HappyPath(t *testing.T) {
	ctx := context.Background()
	url := "http://localhost:8080/rpc"
	jsonRpcMethod := mcpconst.JsonRpcMethod("test/method")
	params := struct {
		Key string `json:"key"`
	}{
		Key: "value",
	}
	additionalHeaders := map[string]string{
		"X-Test-Header": "test-value",
	}

	req, err := NewJSONRPCRequest(ctx, url, jsonRpcMethod, params, additionalHeaders, mockNewHttpRequester)
	require.NoError(t, err)
	require.NotNil(t, req)

	// Check basic request properties
	assert.Equal(t, http.MethodPost, req.Method)
	assert.Equal(t, url, req.URL.String())

	// Check headers
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.Equal(t, "application/json, text/event-stream", req.Header.Get("Accept"))
	assert.Equal(t, "test-value", req.Header.Get("X-Test-Header"))

	// Check the body content
	bodyBytes, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	defer req.Body.Close()

	var rpcReq jsonrpc2.Request
	err = json.Unmarshal(bodyBytes, &rpcReq)
	require.NoError(t, err, "Failed to unmarshal request body into jsonrpc2.Request")

	assert.Equal(t, string(jsonRpcMethod), rpcReq.Method)
	assert.False(t, rpcReq.Notif, "Should not be a notification")
	require.NotNil(t, rpcReq.ID.Num, "ID should be set for a standard request")

	// Verify params
	var decodedParams struct {
		Key string `json:"key"`
	}
	err = json.Unmarshal(*rpcReq.Params, &decodedParams)
	require.NoError(t, err)
	assert.Equal(t, "value", decodedParams.Key)
}

func TestDoRequest_SSE_HappyPath(t *testing.T) {
	// Setup a mock server to return an SSE response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":\"first event\"}\n"))
		_, _ = w.Write([]byte("data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":\"final event\"}\n\n"))
	}))
	defer server.Close()

	// Create a request to the mock server
	req, err := http.NewRequest(http.MethodPost, server.URL, nil)
	require.NoError(t, err)

	// Execute the request
	rpcResp, httpResp, err := DoRequest(context.Background(), server.Client(), req)
	require.NoError(t, err)
	require.NotNil(t, httpResp)
	require.NotNil(t, rpcResp)

	// Verify the response
	assert.Equal(t, http.StatusOK, httpResp.StatusCode)
	require.Nil(t, rpcResp.Error)

	var result string
	err = json.Unmarshal(*rpcResp.Result, &result)
	require.NoError(t, err)
	assert.Equal(t, "final event", result) // TODO make this a const we use above
}

func TestDoRequest_JSON_HappyPath(t *testing.T) {
	// Setup a mock server to return a JSON response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"ok"}`))
	}))
	defer server.Close()

	// Create a request to the mock server
	req, err := http.NewRequest(http.MethodPost, server.URL, nil)
	require.NoError(t, err)

	// Execute the request
	rpcResp, httpResp, err := DoRequest(context.Background(), server.Client(), req)
	require.NoError(t, err)
	require.NotNil(t, httpResp)
	require.NotNil(t, rpcResp)

	// Verify the response
	assert.Equal(t, http.StatusOK, httpResp.StatusCode)
	require.Nil(t, rpcResp.Error)

	var result string
	err = json.Unmarshal(*rpcResp.Result, &result)
	require.NoError(t, err)
	assert.Equal(t, "ok", result)
}

func TestDoRequest_Non2xxError(t *testing.T) {
	// Setup a mock server to return a 500 error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	// Create a request to the mock server
	req, err := http.NewRequest(http.MethodPost, server.URL, nil)
	require.NoError(t, err)

	// Execute the request
	rpcResp, httpResp, err := DoRequest(context.Background(), server.Client(), req)
	require.Error(t, err)
	require.NotNil(t, httpResp)
	require.Nil(t, rpcResp)

	// Verify the error
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unavailable, st.Code())
	assert.Contains(t, st.Message(), "mcp server returned non-2xx status: 500: internal server error")
}
