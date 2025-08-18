package examplemcp

import (
	"context"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/types/known/structpb"
)

type AddParams struct {
	A int `json:"a" jsonschema:"the first number"`
	B int `json:"b" jsonschema:"the second number"`
}

func Add(ctx context.Context, req *mcp.ServerRequest[*mcp.CallToolParamsFor[AddParams]]) (*mcp.CallToolResultFor[any], error) {
	result := req.Params.Arguments.A + req.Params.Arguments.B
	st, err := structpb.NewStruct(map[string]interface{}{"result": result})
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResultFor[any]{
		StructuredContent: st,
	}, nil
}

type MultParams struct {
	A int `json:"a" jsonschema:"the first number"`
	B int `json:"b" jsonschema:"the second number"`
}

func Mult(ctx context.Context, req *mcp.ServerRequest[*mcp.CallToolParamsFor[MultParams]]) (*mcp.CallToolResultFor[any], error) {
	result := req.Params.Arguments.A * req.Params.Arguments.B
	st, err := structpb.NewStruct(map[string]interface{}{"result": result})
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResultFor[any]{
		StructuredContent: st,
	}, nil
}

type LowerParams struct {
	S string `json:"s" jsonschema:"the string to convert to lowercase"`
}

func Lower(ctx context.Context, req *mcp.ServerRequest[*mcp.CallToolParamsFor[LowerParams]]) (*mcp.CallToolResultFor[any], error) {
	result := strings.ToLower(req.Params.Arguments.S)
	st, err := structpb.NewStruct(map[string]interface{}{"result": result})
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResultFor[any]{
		StructuredContent: st,
	}, nil
}

func RunTrivyServer(executableName string) http.Handler {

	server := mcp.NewServer(&mcp.Implementation{Name: executableName, Version: "v1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "add", Description: "add two numbers"}, Add)
	mcp.AddTool(server, &mcp.Tool{Name: "mult", Description: "multiply two numbers"}, Mult)
	mcp.AddTool(server, &mcp.Tool{Name: "lower", Description: "convert a string to lowercase"}, Lower)

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	return handler
}
