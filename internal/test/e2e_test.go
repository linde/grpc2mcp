package test

import (
	"context"
	"grpc2mcp/pb"
	"log"
	"testing"

	asserts "github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestE2e(t *testing.T) {

	assert := asserts.New(t)

	conn, closeFunc, err := SetupMcpAndProxyAsync(t.Name())
	assert.NoError(err)
	defer conn.Close()
	defer closeFunc()

	mcpGrpcClient := pb.NewModelContextProtocolClient(conn)
	ctx := context.Background()

	sessionCtx, err := doInitialize(ctx, mcpGrpcClient)
	assert.NoError(err)
	if err != nil {
		return
	}

	PingResult, err := mcpGrpcClient.Ping(sessionCtx, &pb.PingRequest{})
	assert.NoError(err)
	log.Printf("ping: %v", PingResult)

	listToolsResult, err := mcpGrpcClient.ListTools(sessionCtx, &pb.ListToolsRequest{})
	assert.NoError(err)

	// this tests our ListTools rpc making sure that our target tools are present
	// TODO make the trivy server export these somehow, even statically
targetToolsLoop:
	for _, target := range []string{"add", "mult", "lower"} {
		for _, tool := range listToolsResult.Tools {
			if tool.Name == target {
				log.Printf("ListTools: found target %s", target)
				continue targetToolsLoop
			}
		}
		assert.Failf("ListTools", "missing tool %s", target)
	}

}

func TestToolCall(t *testing.T) {

	assert := asserts.New(t)

	conn, closeFunc, err := SetupMcpAndProxyAsync(t.Name())
	assert.NoError(err)
	defer conn.Close()
	defer closeFunc()

	mcpGrpcClient := pb.NewModelContextProtocolClient(conn)
	ctx := context.Background()

	sessionCtx, err := doInitialize(ctx, mcpGrpcClient)
	assert.NoError(err)
	if err != nil {
		return
	}
	assert.NotNil(sessionCtx)

	callToolRequest := &pb.CallToolRequest{
		Name: "add",
		Arguments: map[string]*structpb.Value{
			"a": structpb.NewNumberValue(10),
			"b": structpb.NewNumberValue(1),
		},
	}
	callMethodResult, err := mcpGrpcClient.CallMethod(sessionCtx, callToolRequest)
	assert.NoError(err)
	assert.NotNil(callMethodResult)
	result := callMethodResult.GetStructuredContent().GetFields()["result"].GetNumberValue()
	assert.Equal(float64(11), result)

}

func TestAllTools(t *testing.T) {
	assert := asserts.New(t)

	conn, closeFunc, err := SetupMcpAndProxyAsync(t.Name())
	assert.NoError(err)
	defer conn.Close()
	defer closeFunc()

	mcpGrpcClient := pb.NewModelContextProtocolClient(conn)
	ctx := context.Background()

	sessionCtx, err := doInitialize(ctx, mcpGrpcClient)
	if err != nil {
		assert.NoError(err)
		t.FailNow()
	}

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
		t.Run(tc.tool, func(t *testing.T) {
			assert := asserts.New(t)

			argsStruct, err := structpb.NewStruct(tc.args)
			assert.NoError(err)

			callToolRequest := &pb.CallToolRequest{
				Name:      tc.tool,
				Arguments: argsStruct.GetFields(),
			}
			callMethodResult, err := mcpGrpcClient.CallMethod(sessionCtx, callToolRequest)
			assert.NoError(err)
			assert.NotNil(callMethodResult)

			resultField := callMethodResult.GetStructuredContent().GetFields()["result"]
			assert.NotNil(resultField)

			switch expected := tc.expected.(type) {
			case float64:
				assert.Equal(expected, resultField.GetNumberValue())
			case string:
				assert.Equal(expected, resultField.GetStringValue())
			default:
				t.Fatalf("unhandled expected type: %T", expected)
			}
		})
	}
}
