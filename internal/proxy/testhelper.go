package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"grpc2mcp/internal/examplemcp"
	"grpc2mcp/internal/mcpconst"
	"grpc2mcp/pb"
	"log"
	"net/http/httptest"
	"sort"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"
)

// This file contains tests that we run e2e via network gRPC and also directly
// in process via bufcon.

func doGrpcProxyTests(t *testing.T, mcpGrpcClient pb.ModelContextProtocolClient) {

	sessionCtx, err := doProxyInitialize(t.Context(), mcpGrpcClient)
	require.NoError(t, err)

	PingResult, err := mcpGrpcClient.Ping(sessionCtx, &pb.PingRequest{})
	require.NoErrorf(t, err, "error with Ping")
	log.Printf("ping: %v", PingResult)

	// this tests our ListTools rpc making sure that our target tools are present
	toolsExpected := examplemcp.GetProvidedToolNames()

	toolsFound := []string{}
	listToolsResult, err := mcpGrpcClient.ListTools(sessionCtx, &pb.ListToolsRequest{})
	require.NoErrorf(t, err, "error with ListTools")

	for _, tool := range listToolsResult.Tools {
		toolsFound = append(toolsFound, tool.Name)
	}
	sort.Strings(toolsFound)

	// TODO this shoud probably just be an assert.ElementsMatch()

	assert := assert.New(t)
	assert.EqualValuesf(toolsExpected, toolsFound, "set of tools not what was expected")
}

type ToolTestData struct {
	tool     string
	args     map[string]any
	expected string
	isError  bool
}

func (ttd ToolTestData) NewToolRequest() (*pb.CallToolRequest, error) {
	argsStruct, err := structpb.NewStruct(ttd.args)
	if err != nil {
		return nil, fmt.Errorf("error making NewStruct: %v", err)
	}

	callToolRequest := &pb.CallToolRequest{
		Name:      ttd.tool,
		Arguments: argsStruct.GetFields(),
	}

	return callToolRequest, nil
}

// used in streaming tests too
var toolTestData = []ToolTestData{
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
}

func doGrpcProxyToolTests(t *testing.T, mcpGrpcClient pb.ModelContextProtocolClient) {

	assert := assert.New(t)

	sessionCtx, err := doProxyInitialize(t.Context(), mcpGrpcClient)
	assert.NoErrorf(err, "error with doProxyInitialize")

	for idx, ttd := range toolTestData {
		log.Printf("CallMethod (%d of %d): %s %v", idx, len(toolTestData), ttd.tool, ttd.args)

		callToolRequest, err := ttd.NewToolRequest()
		assert.NoErrorf(err, "error with NewToolRequest")

		callToolResult, err := mcpGrpcClient.CallMethod(sessionCtx, callToolRequest)
		assert.NoErrorf(err, "error with CallMethod")

		validateCallToolResult(t, callToolResult, ttd)

	}
}

func validateCallToolResult(t *testing.T, callToolResult *pb.CallToolResult, ttd ToolTestData) {
	assert := assert.New(t)

	assert.NotNil(callToolResult)
	assert.NotEmpty(callToolResult.GetContent(), "callToolResult content array was empty")

	assert.Equal(ttd.isError, callToolResult.GetIsError(), "unexpected callMethodResult error status")
	if callToolResult.GetIsError() {
		return // Continue to the next test case
	}

	cb := callToolResult.GetContent()[0]

	if resultValue, ok := getResultFromContentBlock(cb); ok {
		assert.EqualValues(ttd.expected, resultValue, "invalid callMethodResult")
	} else {
		assert.Fail("unexpected type for content block: %T", cb.GetContentType())
	}

}

// assumes an already validated calltoolresult
func getResultFromContentBlock(cb *pb.ContentBlock) (any, bool) {

	switch c := cb.ContentType.(type) {
	case *pb.ContentBlock_Text:
		return c.Text.Text, true

	case *pb.ContentBlock_ResourceLink:
		return c.ResourceLink.Resource.Uri, true

	default:
		log.Printf("unexpected content type from GetContent(): %T", c)
		return nil, false
	}

}

func doGrpcProxyResourceTests(t *testing.T, mcpGrpcClient pb.ModelContextProtocolClient) {

	assert := assert.New(t)

	sessionCtx, err := doProxyInitialize(t.Context(), mcpGrpcClient)
	assert.NoErrorf(err, "error with doProxyInitialize")

	listResourcesResult, err := mcpGrpcClient.ListResources(sessionCtx, &pb.ListResourcesRequest{})
	assert.NoErrorf(err, "error with ListResources")

	var resourceNamesExpected []string
	for _, resourceProvided := range examplemcp.ResourcesProvided {
		resourceNamesExpected = append(resourceNamesExpected, resourceProvided.GetName())
	}

	var resourceNamesProvided []string
	for _, r := range listResourcesResult.GetResources() {
		resourceNamesProvided = append(resourceNamesProvided, r.Name)
	}

	assert.Equalf(resourceNamesExpected, resourceNamesProvided, "resource names mis matched")

}

func doGrpcProxyPromptTests(t *testing.T, mcpGrpcClient pb.ModelContextProtocolClient) {

	assert := assert.New(t)

	sessionCtx, err := doProxyInitialize(t.Context(), mcpGrpcClient)
	assert.NoErrorf(err, "error with doProxyInitialize")

	listPromptResult, err := mcpGrpcClient.ListPrompts(sessionCtx, &pb.ListPromptsRequest{})
	assert.NoErrorf(err, "error with ListPrompts")

	promptNamesExpected := examplemcp.GetProvidedPrompts()

	var promptNamesProvided []string
	for _, p := range listPromptResult.GetPrompts() {
		promptNamesProvided = append(promptNamesProvided, p.Name)
	}

	assert.Equalf(promptNamesExpected, promptNamesProvided, "prompt names mis matched")

	// TODO make some assertions about the prompt
	_, err = mcpGrpcClient.GetPrompt(sessionCtx, &pb.GetPromptRequest{Name: promptNamesExpected[0]})
	assert.NoErrorf(err, "error with GetPrompt")

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

func doProxyInitialize(ctx context.Context, mcpGrpcClient pb.ModelContextProtocolClient) (context.Context, error) {

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

	clientCtx := metadata.AppendToOutgoingContext(ctx, mcpconst.MCP_SESSION_ID_HEADER, mcpSessionId[0])
	return clientCtx, nil

}

func doGrpcProxyStreamTests(t *testing.T, mcpGrpcClient pb.ModelContextProtocolClient) {

	assert := assert.New(t)
	sessionCtx, err := doProxyInitialize(t.Context(), mcpGrpcClient)
	require.NoErrorf(t, err, "error with doMcpInitialize")

	// first do the send part, stream the contents of an array to the server

	stream, err := mcpGrpcClient.CallMethodStream(sessionCtx)
	require.NoErrorf(t, err, "error with CallMethodStream")

	for sendCount, callToolReq := range toolTestData {

		callToolRequest, err := callToolReq.NewToolRequest()
		assert.NoErrorf(err, "error with NewToolRequest on sendCount %d", sendCount)

		err = stream.Send(callToolRequest)
		if err == io.EOF {
			break
		}
		require.NoErrorf(t, err, "error on stream.Send number %d", sendCount)
	}

	stream.CloseSend()

	// now walk through the responses

	numResultsReceived := 0
	for { // TODO consider a select that times out

		callToolResult, err := stream.Recv()
		if err == io.EOF {
			log.Printf("got EOF in stream.Recv")
			break
		}
		numResultsReceived += 1

		require.NoErrorf(t, err, "error on stream.Recv")

		// subtrack one bc toolTestData is zero indexed but results received is one indexed
		curTest := toolTestData[numResultsReceived-1]

		validateCallToolResult(t, callToolResult, curTest)
	}

	assert.Equal(len(toolTestData), numResultsReceived)

}

func doMcpClientTests(t *testing.T, mcpGrpcClient pb.ModelContextProtocolClient) {

	doGrpcProxyTests(t, mcpGrpcClient)
	doGrpcProxyToolTests(t, mcpGrpcClient)
	doGrpcProxyPromptTests(t, mcpGrpcClient)
	doGrpcProxyResourceTests(t, mcpGrpcClient)
	doGrpcProxyStreamTests(t, mcpGrpcClient)

}
