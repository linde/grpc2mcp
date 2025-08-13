package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func parseJsonRpcResponseBody(body []byte) (map[string]string, error) {

	parsedData := make(map[string]string)
	reader := bytes.NewReader(body)
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			// Skip malformed lines, but you could also return an error.
			fmt.Printf("Skipping malformed line: %s\n", line)
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		parsedData[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading stream: %w", err)
	}

	return parsedData, nil
}

func NewJSONRPCRequest(ctx context.Context, host string, port int, uri string,
	method string, params any) (*http.Request, error) {

	jsonRPCReq := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
	}

	if params != nil {
		jsonRPCReq.Params = params
		jsonRPCReq.ID = rand.Int() // TODO is this safe to assume ok to ad ID?
	}

	jsonRPCReqBytes, err := json.Marshal(jsonRPCReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to marshal json-rpc request: %v", err)
	}

	url := fmt.Sprintf("http://%s:%d%s", host, port, uri)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonRPCReqBytes))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create http request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")

	return httpReq, nil
}

// JSONRPCRequest represents a JSON-RPC request object.
type JSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
	ID      int    `json:"id,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC response object.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *JSONRPCError   `json:"error"`
	ID      int64           `json:"id"`
}

// JSONRPCError represents the error object in a JSON-RPC response.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// // ClientCapabilities represents the capabilities of the client.
// type ClientCapabilities struct{}

// // Implementation represents the client implementation information.
// type Implementation struct {
// 	Name    string `json:"name"`
// 	Version string `json:"version"`
// }

// // InitializeParams represents the parameters for the 'initialize' request.
// type InitializeParams struct {
// 	ProtocolVersion string             `json:"protocolVersion"`
// 	Capabilities    ClientCapabilities `json:"capabilities"`
// 	ClientInfo      Implementation     `json:"clientInfo"`
// }
