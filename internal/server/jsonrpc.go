package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
		// TODO is this safe to assume ok to add ID when there are params?
		jsonRPCReq.ID = rand.Int()
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

func getJSONRPCRequestResponse(ctx context.Context,
	host string, port int, uri string, method string, paramSrc any, headers map[string]string) (map[string]string, error) {

	var params map[string]any

	if paramSrc != nil {
		// Marshal the protobuf Struct to JSON bytes
		paramsBytes, err := json.Marshal(paramSrc)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to marshal params to json: %v", err)
		}
		// Unmarshal the JSON bytes into a map[string]any
		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to unmarshal params to map: %v", err)
		}
	} else {
		params = nil
	}

	httpReq, err := NewJSONRPCRequest(ctx, host, port, uri, method, params)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create http request: %v", err)
	}
	for header, value := range headers {
		httpReq.Header.Set(header, value)
	}

	client := http.Client{}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "failed to call mcp server: %v", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read mcp server response: %v", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, status.Errorf(codes.Unavailable, "mcp server returned non-200 status: %d", httpResp.StatusCode)
	}

	jsonRpcResponseParts, err := parseJsonRpcResponseBody(respBody)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to parse mcp server response: %v", err)
	}

	return jsonRpcResponseParts, nil
}
