package examplemcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"grpc2mcp/internal/jsonrpc"
	"grpc2mcp/internal/mcpconst"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testNewRequester(method string, url string, body io.Reader) (*http.Request, error) {
	return httptest.NewRequest(method, url, body), nil
}

func setupSession(t *testing.T, handler http.Handler) string {
	assert := assert.New(t)

	// 1. Initialize
	initParams := map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "go-test",
			"version": "1.0",
		},
	}

	addlHeaders := map[string]string{} // no headers
	initReq, err := jsonrpc.NewJSONRPCRequest("/", mcpconst.Initialize,
		initParams, addlHeaders, testNewRequester)
	assert.NoError(err)

	initRR := httptest.NewRecorder()
	handler.ServeHTTP(initRR, initReq)
	assert.Equal(http.StatusOK, initRR.Code)

	sessionID := initRR.Header().Get(mcpconst.MCP_SESSION_ID_HEADER)
	assert.NotEmpty(sessionID)

	// 2. Initialized
	addlHeaders = map[string]string{mcpconst.MCP_SESSION_ID_HEADER: sessionID}
	initializedReq, err := jsonrpc.NewJSONRPCRequest("/",
		mcpconst.NotificationsInitialized, nil, addlHeaders, testNewRequester)
	assert.NoError(err)

	initializedRR := httptest.NewRecorder()
	handler.ServeHTTP(initializedRR, initializedReq)
	assert.Equal(http.StatusAccepted, initializedRR.Code)

	return sessionID
}

func TestTrivyServerTools(t *testing.T) {
	handler := RunTrivyServer("test")
	sessionID := setupSession(t, handler)

	testCases := []struct {
		name             string
		tool             string
		input            any
		expectedResponse map[string]any
	}{
		{
			name:             "add two numbers",
			tool:             "add",
			input:            AddParams{A: 5, B: 3},
			expectedResponse: map[string]any{"result": 8.0},
		},
		{
			name:             "multiply two numbers",
			tool:             "mult",
			input:            MultParams{A: 5, B: 3},
			expectedResponse: map[string]any{"result": 15.0},
		},
		{
			name:             "convert string to lowercase",
			tool:             "lower",
			input:            LowerParams{S: "HELLO"},
			expectedResponse: map[string]any{"result": "hello"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			params := map[string]any{
				"name":      tc.tool,
				"arguments": tc.input,
			}

			addlHeaders := map[string]string{mcpconst.MCP_SESSION_ID_HEADER: sessionID}
			req, err := jsonrpc.NewJSONRPCRequest("/", mcpconst.ToolsCall, params,
				addlHeaders, testNewRequester)
			assert.NoError(err)

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			assert.Equal(http.StatusOK, rr.Code)

			respBody, err := io.ReadAll(rr.Body)
			assert.NoError(err)

			// The response is a server-sent event stream. We need to parse it.
			scanner := bufio.NewScanner(bytes.NewReader(respBody))
			var eventData string
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "data: ") {
					eventData = strings.TrimPrefix(line, "data: ")
				}
			}
			assert.NoError(scanner.Err())

			var result struct {
				Result json.RawMessage `json:"result"`
			}
			err = json.Unmarshal([]byte(eventData), &result)
			assert.NoError(err)

			var structuredContent struct {
				StructuredContent map[string]any `json:"structuredContent"`
			}
			err = json.Unmarshal(result.Result, &structuredContent)
			assert.NoError(err)

			assert.Equal(tc.expectedResponse["result"], structuredContent.StructuredContent["result"])
		})
	}
}
