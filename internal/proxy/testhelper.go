package proxy

import (
	"context"
	"errors"
	"fmt"
	"grpc2mcp/internal/examplemcp"
	"grpc2mcp/internal/mcpconst"
	"grpc2mcp/pb"
	"log"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"
)

// This file contains tests that we run e2e via network gRPC and also directly
// in process via bufcon.

func doGrpcProxyTests(ctx context.Context, mcpGrpcClient pb.ModelContextProtocolClient) error {

	sessionCtx, err := doMcpInitialize(ctx, mcpGrpcClient)
	if err != nil {
		return fmt.Errorf("error with initiailization: %w", err)
	}

	PingResult, err := mcpGrpcClient.Ping(sessionCtx, &pb.PingRequest{})
	if err != nil {
		return fmt.Errorf("error with Ping: %w", err)
	}
	log.Printf("ping: %v", PingResult)

	// this tests our ListTools rpc making sure that our target tools are present
	toolsExpected := []string{}
	for _, providedTool := range examplemcp.ToolsProvided {
		toolsExpected = append(toolsExpected, providedTool.GetName())
	}
	sort.Strings(toolsExpected)

	toolsFound := []string{}
	listToolsResult, err := mcpGrpcClient.ListTools(sessionCtx, &pb.ListToolsRequest{})
	if err != nil {
		return fmt.Errorf("error with ListTools: %w", err)
	}
	for _, tool := range listToolsResult.Tools {
		toolsFound = append(toolsFound, tool.Name)
	}
	sort.Strings(toolsFound)

	// TODO this shoud probably just be an assert.ElementsMatch()
	if !reflect.DeepEqual(toolsExpected, toolsFound) {
		return fmt.Errorf("tools expected %v not equal tools found %v", toolsExpected, toolsFound)
	}
	return nil
}

func doGrpcProxyToolTests(ctx context.Context, mcpGrpcClient pb.ModelContextProtocolClient) error {

	sessionCtx, err := doMcpInitialize(ctx, mcpGrpcClient)
	if err != nil {
		return fmt.Errorf("error making NewStruct: %v", err)
	}

	failedAssertions := []string{}

	for _, tc := range []struct {
		tool     string
		args     map[string]any
		expected any
	}{
		{"add", map[string]any{"a": 1, "b": 10}, 11.0},
		{"add", map[string]any{"a": 1}, 1.0},
		{"add", map[string]any{"b": 1}, 1.0},
		{"add", map[string]any{"a": 0, "b": 10}, 10.0},
		{"add", map[string]any{"a": 1, "b": -10}, -9.0},
		{"mult", map[string]any{"a": 11, "b": 3}, 33.0},
		{"mult", map[string]any{"a": 0, "b": 3}, 0.0},
		{"lower", map[string]any{"s": "MixedCase"}, "mixedcase"},
		{"lower", map[string]any{"s": ""}, ""},
		{"lower", map[string]any{}, ""},
	} {
		log.Printf("CallMethod: %s %v", tc.tool, tc.args)

		argsStruct, err := structpb.NewStruct(tc.args)
		if err != nil {
			return fmt.Errorf("error making NewStruct: %v", err)
		}

		callToolRequest := &pb.CallToolRequest{
			Name:      tc.tool,
			Arguments: argsStruct.GetFields(),
		}
		callMethodResult, err := mcpGrpcClient.CallMethod(sessionCtx, callToolRequest)
		if err != nil {
			return fmt.Errorf("error with CallMethod(): %v", err)
		}
		if callMethodResult == nil {
			return fmt.Errorf("nil method result from CallMethod()")
		}

		resultField := callMethodResult.GetStructuredContent().GetFields()["result"]
		if resultField == nil {
			return fmt.Errorf("nil resultField from CallMethod()")
		}

		switch expected := tc.expected.(type) {
		case float64:
			if expected != resultField.GetNumberValue() {
				failStr := fmt.Sprintf("%s %v, exepected %v, observed %v", tc.tool, tc.args, expected, resultField.GetNumberValue())
				failedAssertions = append(failedAssertions, failStr)
			}
		case string:
			if expected != resultField.GetStringValue() {
				failStr := fmt.Sprintf("%s %v, exepected %v, observed %v", tc.tool, tc.args, expected, resultField.GetStringValue())
				failedAssertions = append(failedAssertions, failStr)
			}
		default:
			failStr := fmt.Sprintf("%s %v, unhandled type %T", tc.tool, tc.args, expected)
			failedAssertions = append(failedAssertions, failStr)
		}
	}

	if len(failedAssertions) > 0 {
		return fmt.Errorf("doGrpcProxyToolTests failed %d assertions:\n%s",
			len(failedAssertions), strings.Join(failedAssertions, "\n"))
	}
	return nil
}

func doGrpcProxyPromptTests(ctx context.Context, mcpGrpcClient pb.ModelContextProtocolClient) error {

	sessionCtx, err := doMcpInitialize(ctx, mcpGrpcClient)
	if err != nil {
		return err
	}

	listPromptResult, err := mcpGrpcClient.ListPrompts(sessionCtx, &pb.ListPromptsRequest{})
	if err != nil {
		return err
	}

	var promptNamesExpected []string
	for name := range examplemcp.PromptsProvided {
		promptNamesExpected = append(promptNamesExpected, name)
	}

	var promptNamesProvided []string
	for _, p := range listPromptResult.GetPrompts() {
		promptNamesProvided = append(promptNamesProvided, p.Name)
	}

	if !reflect.DeepEqual(promptNamesExpected, promptNamesProvided) {
		return fmt.Errorf("tools expected %v not equal tools found %v", promptNamesExpected, promptNamesProvided)
	}

	// TODO make some assertions about the prompt
	_, err = mcpGrpcClient.GetPrompt(sessionCtx, &pb.GetPromptRequest{Name: promptNamesExpected[0]})
	if err != nil {
		return err
	}

	return nil
}

func SetupAsyncMcpAndProxy(mcpServerName string) (pb.ModelContextProtocolClient, func(), error) {

	closeLine := &CloseLine{}

	handler := examplemcp.RunTrivyServer(mcpServerName)
	ts := httptest.NewServer(handler)
	closeLine.Add(ts.Close)
	log.Printf("mcp handler listening on: %s", ts.URL)
	s, err := NewServer(ts.URL)
	if err != nil {
		closeLine.Close()
		return nil, closeLine.Close, fmt.Errorf("failed to create proxy server: %w", err)
	}

	proxyTcpAddr, proxyCancelFunc, err := s.StartAsync(0)
	if err != nil {
		closeLine.Close()
		return nil, closeLine.Close, fmt.Errorf("failed to start proxy server: %w", err)
	}
	closeLine.Add(proxyCancelFunc)

	log.Printf("mcp grpc proxy listening on: %s", proxyTcpAddr)

	// put together a client and return it to the caller
	newClient, newClientErr := grpc.NewClient(proxyTcpAddr.String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if newClientErr != nil {
		closeLine.Close()
		return nil, closeLine.Close, newClientErr
	}
	closeLine.AddE(newClient.Close)

	mcpGrpcClient := pb.NewModelContextProtocolClient(newClient)

	return mcpGrpcClient, closeLine.Close, nil

}

func doMcpInitialize(ctx context.Context, mcpGrpcClient pb.ModelContextProtocolClient) (context.Context, error) {

	var sessionHeader metadata.MD
	_, err := mcpGrpcClient.Initialize(ctx, &pb.InitializeRequest{}, grpc.Header(&sessionHeader))
	if err != nil {
		return nil, fmt.Errorf("error making Initialize grpc call: %w", err)
	}

	mcpSessionId := sessionHeader.Get(mcpconst.MCP_SESSION_ID_HEADER)
	if len(mcpSessionId) < 1 {
		errStr := fmt.Sprintf("did not receive mcp session id: %s", mcpconst.MCP_SESSION_ID_HEADER)
		return nil, errors.New(errStr)
	}
	clientCtx := metadata.NewOutgoingContext(context.Background(), sessionHeader)

	return clientCtx, nil

}
