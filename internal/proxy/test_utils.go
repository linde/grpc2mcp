package proxy

import (
	"context"
	"fmt"
	"grpc2mcp/internal/examplemcp"
	"grpc2mcp/pb"
	"log"
	"strings"

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

	listToolsResult, err := mcpGrpcClient.ListTools(sessionCtx, &pb.ListToolsRequest{})
	if err != nil {
		return fmt.Errorf("error with ListTools: %w", err)
	}
	// this tests our ListTools rpc making sure that our target tools are present
	missingTools := []string{}
targetToolsLoop:
	for _, providedTool := range examplemcp.ToolsProvided {
		target := providedTool.GetName()

		for _, tool := range listToolsResult.Tools {
			if tool.Name == target {
				log.Printf("ListTools: found target %s", target)
				continue targetToolsLoop
			}
		}
		missingTools = append(missingTools, target)
	}

	if len(missingTools) > 0 {
		return fmt.Errorf("error ListTools missing: %v", missingTools)
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
