package jsonrpc

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
	"google.golang.org/protobuf/proto"
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

// NewBody creates the body of a JSON-RPC request.
func NewBody(method string, params any) map[string]any {
	body := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if !strings.HasPrefix(method, "notifications/") {
		body["id"] = rand.Int()
	}
	if params != nil {
		body["params"] = params
	}
	return body
}

func NewJSONRPCRequest(host string, port int, uri string,
	method string, params proto.Message, additionalHeaders map[string]string,
) (*http.Request, error) {

	reqBody := NewBody(method, params)
	jsonRPCReqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to marshal json-rpc request: %v", err)
	}

	url := fmt.Sprintf("http://%s:%d%s", host, port, uri)
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonRPCReqBytes))

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create http request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")

	for header, val := range additionalHeaders {
		httpReq.Header.Set(header, val)
	}

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

func GetJSONRPCRequestResponse(ctx context.Context,
	host string, port int, uri string, method string,
	paramSrc proto.Message, headers map[string]string) (map[string]string, error) {

	httpReq, err := NewJSONRPCRequest(host, port, uri, method, paramSrc, nil)
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

// this allows us to leverage different ways to create the http Request, a normal
// network one or also a mock one for testing. we set headers and deal with the body
// the same either way in NewJSONRPCRequest()
type NewHttpRequester func(string, string, io.Reader) *http.Request

// This function consolidates request manipulation for a JSONRPC request. it allows
// the caller to pass in the request constructor so we can use a mock in tests
func NewJSONRPCRequest2(url string, method string, params any,
	additionalHeaders map[string]string, reqFunc NewHttpRequester) (*http.Request, error) {

	// TODO general sessionID into additionalHeaders also return the error

	reqBody := NewBody(method, params)

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("error putting together jsonrpc request: %w", err)
	}

	req := reqFunc(http.MethodPost, url, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	for header, val := range additionalHeaders {
		req.Header.Set(header, val)
	}

	return req, nil
}
