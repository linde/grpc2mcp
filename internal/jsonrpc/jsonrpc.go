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

	"grpc2mcp/internal/mcpconst"

	"github.com/sourcegraph/jsonrpc2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// this allows us to leverage different ways to create the http Request, a normal
// network one or also a mock one for testing. we set headers and deal with the body
// the same either way in NewJSONRPCRequest()
type NewHttpRequester func(ctx context.Context, method string, url string, body io.Reader) (*http.Request, error)

// This function consolidates request manipulation for a JSONRPC request. it allows
// the caller to pass in the request constructor so we can use a mock in tests
func NewJSONRPCRequest(ctx context.Context, url string, jsonRpcMethod mcpconst.JsonRpcMethod, params any,
	additionalHeaders map[string]string, reqFunc NewHttpRequester) (*http.Request, error) {

	var rawParams *json.RawMessage
	if params != nil {
		paramsMsg, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal params: %w", err)
		}
		rawParams = (*json.RawMessage)(&paramsMsg)
	}

	isNotification := strings.HasPrefix(string(jsonRpcMethod), "notifications/")
	reqBody := &jsonrpc2.Request{
		Method: string(jsonRpcMethod),
		Params: rawParams,
		ID:     jsonrpc2.ID{Num: uint64(rand.Int63())},
		Notif:  isNotification,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("error putting together jsonrpc request: %w", err)
	}

	req, err := reqFunc(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("problem creating new JSONRPC request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	for header, val := range additionalHeaders {
		req.Header.Set(header, val)
	}

	return req, nil
}

// DoRequest sends a JSON-RPC request and handles parsing the response, correctly
// interpreting both standard JSON and SSE (text/event-stream) formats.
func DoRequest(ctx context.Context, client *http.Client, req *http.Request) (*jsonrpc2.Response, *http.Response, error) {
	httpResp, err := client.Do(req)
	if err != nil {
		return nil, nil, status.Errorf(codes.Unavailable, "failed to call mcp server: %v", err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		body, _ := io.ReadAll(httpResp.Body)
		_ = httpResp.Body.Close()
		return nil, httpResp, status.Errorf(codes.Unavailable, "mcp server returned non-2xx status: %d: %s", httpResp.StatusCode, string(body))
	}

	contentType := httpResp.Header.Get("Content-Type")
	var respBody []byte

	if strings.Contains(contentType, "text/event-stream") {
		// For SSE, we scan for the last "data:" line.
		scanner := bufio.NewScanner(httpResp.Body)
		var lastData string
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				lastData = strings.TrimPrefix(line, "data: ")
			}
		}
		if err := scanner.Err(); err != nil {
			_ = httpResp.Body.Close()
			return nil, httpResp, status.Errorf(codes.Internal, "failed to read mcp server SSE response: %v", err)
		}
		if lastData == "" {
			// It's possible to get a 200 OK with an empty stream, e.g., for notifications.
			// Return success with a nil response object.
			_ = httpResp.Body.Close()
			return nil, httpResp, nil
		}
		respBody = []byte(lastData)
	} else {
		// For application/json, read the whole body.
		body, err := io.ReadAll(httpResp.Body)
		if err != nil {
			_ = httpResp.Body.Close()
			return nil, httpResp, status.Errorf(codes.Internal, "failed to read mcp server response: %v", err)
		}
		respBody = body
	}

	// It's important to close the body after we're done reading it.
	_ = httpResp.Body.Close()

	if len(respBody) == 0 {
		// Handle cases where the body is empty but the status was OK.
		return nil, httpResp, nil
	}

	var resp jsonrpc2.Response
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, httpResp, status.Errorf(codes.Internal, "failed to unmarshal mcp server response: %s", string(respBody))
	}

	return &resp, httpResp, nil
}
