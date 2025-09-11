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

// TODO just go ahead and pass a test T in here

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
	toolsExpected := examplemcp.GetProvidedToolNames()

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
		expected string
		isError  bool
	}{
		{examplemcp.TOOL_ADD, map[string]any{examplemcp.PARAM_A: 1, examplemcp.PARAM_B: 10}, "11", false},
		{examplemcp.TOOL_ADD, map[string]any{examplemcp.PARAM_A: 0, examplemcp.PARAM_B: 10}, "10", false},
		{examplemcp.TOOL_ADD, map[string]any{examplemcp.PARAM_A: 10, examplemcp.PARAM_B: 0}, "10", false},
		{examplemcp.TOOL_ADD, map[string]any{examplemcp.PARAM_A: -10, examplemcp.PARAM_B: 5}, "-5", false},
		{examplemcp.TOOL_ADD, map[string]any{examplemcp.PARAM_A: 5, examplemcp.PARAM_B: -10}, "-5", false},

		{examplemcp.TOOL_ADD, map[string]any{examplemcp.PARAM_A: 1}, fmt.Sprintf(`required argument "%s" not found`, examplemcp.PARAM_B), true},
		{examplemcp.TOOL_ADD, map[string]any{examplemcp.PARAM_B: 1}, fmt.Sprintf(`required argument "%s" not found`, examplemcp.PARAM_A), true},

		{examplemcp.TOOL_MULT, map[string]any{examplemcp.PARAM_A: 1, examplemcp.PARAM_B: 10}, "10", false},
		{examplemcp.TOOL_MULT, map[string]any{examplemcp.PARAM_A: 0, examplemcp.PARAM_B: 10}, "0", false},
		{examplemcp.TOOL_MULT, map[string]any{examplemcp.PARAM_A: 10, examplemcp.PARAM_B: 0}, "0", false},
		{examplemcp.TOOL_MULT, map[string]any{examplemcp.PARAM_A: -10, examplemcp.PARAM_B: 5}, "-50", false},
		{examplemcp.TOOL_MULT, map[string]any{examplemcp.PARAM_A: 5, examplemcp.PARAM_B: -10}, "-50", false},

		{examplemcp.TOOL_MULT, map[string]any{examplemcp.PARAM_A: 1}, fmt.Sprintf(`required argument "%s" not found`, examplemcp.PARAM_B), true},
		{examplemcp.TOOL_MULT, map[string]any{examplemcp.PARAM_B: 1}, fmt.Sprintf(`required argument "%s" not found`, examplemcp.PARAM_A), true},

		{examplemcp.TOOL_LOWER, map[string]any{examplemcp.PARAM_S: "MixedCase"}, "mixedcase", false},
		{examplemcp.TOOL_LOWER, map[string]any{examplemcp.PARAM_S: ""}, "", false},

		{examplemcp.TOOL_LOWER, map[string]any{}, fmt.Sprintf(`required argument "%s" not found`, examplemcp.PARAM_S), true},

		{examplemcp.TOOL_GREET_RESOURCE, map[string]any{examplemcp.PARAM_WHOM: "yer mom"}, "data:Hello%2C+yer+mom%21", false},
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

		if callMethodResult.GetIsError() != tc.isError {
			failStr := fmt.Sprintf("%s %v, expected isError %v, observed %v", tc.tool, tc.args, tc.isError, callMethodResult.GetIsError())
			failedAssertions = append(failedAssertions, failStr)
			continue // Continue to the next test case
		}

		contentItems := callMethodResult.GetContent()

		if len(contentItems) != 1 {
			// Only fail if we are not expecting an error. Error responses might not have content.
			if !tc.isError {
				return fmt.Errorf("expected 1 content item, got %d", len(contentItems))
			}
			// If we expect an error and there's no content, that's fine.
			if len(contentItems) == 0 {
				continue
			}
		}

		// TODO add other tests for other content types: ImageContent, AudioContent or a ResourceLink
		switch c := contentItems[0].ContentType.(type) {
		case *pb.ContentBlock_Text:
			actualResult := c.Text.Text
			if actualResult != tc.expected {
				failStr := fmt.Sprintf("%s %v, exepected %v, observed %v", tc.tool, tc.args, tc.expected, actualResult)
				failedAssertions = append(failedAssertions, failStr)
			}
		case *pb.ContentBlock_ResourceLink:
			actualResult := c.ResourceLink.Resource.Uri
			if actualResult != tc.expected {
				failStr := fmt.Sprintf("%s %v, exepected %v, observed %v", tc.tool, tc.args, tc.expected, actualResult)
				failedAssertions = append(failedAssertions, failStr)
			}
		default:
			return fmt.Errorf("unexpected content type: %T", c)
		}
	}

	if len(failedAssertions) > 0 {
		return fmt.Errorf("doGrpcProxyToolTests failed %d assertions:\n%s",
			len(failedAssertions), strings.Join(failedAssertions, "\n"))
	}
	return nil
}

// TODO these should be done in the e2e as well, not just bufcon
func doGrpcProxyResourceTests(ctx context.Context, mcpGrpcClient pb.ModelContextProtocolClient) error {

	sessionCtx, err := doMcpInitialize(ctx, mcpGrpcClient)
	if err != nil {
		return err
	}

	listResourcesResult, err := mcpGrpcClient.ListResources(sessionCtx, &pb.ListResourcesRequest{})
	if err != nil {
		return err
	}

	var resourceNamesExpected []string
	for _, resourceProvided := range examplemcp.ResourcesProvided {
		resourceNamesExpected = append(resourceNamesExpected, resourceProvided.GetName())
	}

	var resourceNamesProvided []string
	for _, r := range listResourcesResult.GetResources() {
		resourceNamesProvided = append(resourceNamesProvided, r.Name)
	}

	if !reflect.DeepEqual(resourceNamesExpected, resourceNamesProvided) {
		return fmt.Errorf("resources expected %v not equal resources found %v", resourceNamesExpected, resourceNamesProvided)
	}

	return nil
}

// TODO these should be done in the e2e as well, not just bufcon
func doGrpcProxyPromptTests(ctx context.Context, mcpGrpcClient pb.ModelContextProtocolClient) error {

	sessionCtx, err := doMcpInitialize(ctx, mcpGrpcClient)
	if err != nil {
		return err
	}

	listPromptResult, err := mcpGrpcClient.ListPrompts(sessionCtx, &pb.ListPromptsRequest{})
	if err != nil {
		return err
	}

	promptNamesExpected := examplemcp.GetProvidedPrompts()

	var promptNamesProvided []string
	for _, p := range listPromptResult.GetPrompts() {
		promptNamesProvided = append(promptNamesProvided, p.Name)
	}

	if !reflect.DeepEqual(promptNamesExpected, promptNamesProvided) {
		return fmt.Errorf("prompts expected %v not equal tools found %v", promptNamesExpected, promptNamesProvided)
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

	handler := examplemcp.RunExampleMcpServer(mcpServerName, "/mcp")
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
