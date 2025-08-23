package examplemcp

import (
	"context"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/types/known/structpb"
)

// --- Tool Implementations (Add, Mult, Lower) are unchanged ---
type AddParams struct {
	A int `json:"a" jsonschema:"the first number"`
	B int `json:"b" jsonschema:"the second number"`
}

func Add(ctx context.Context, req *mcp.ServerRequest[*mcp.CallToolParamsFor[AddParams]]) (*mcp.CallToolResultFor[any], error) {
	result := req.Params.Arguments.A + req.Params.Arguments.B
	st, err := structpb.NewStruct(map[string]any{"result": result})
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
	st, err := structpb.NewStruct(map[string]any{"result": result})
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
	st, err := structpb.NewStruct(map[string]any{"result": result})
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResultFor[any]{
		StructuredContent: st,
	}, nil
}

type toolRegisterer interface {
	Register(server *mcp.Server)
	GetName() string
}

type toolDefinition[In, Out any] struct {
	Name        string
	Description string
	Handler     mcp.ToolHandlerFor[In, Out]
}

func (td toolDefinition[In, Out]) Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{Name: td.Name, Description: td.Description}, td.Handler)
}

func (td toolDefinition[In, Out]) GetName() string {
	return td.Name
}

var ToolsProvided []toolRegisterer = []toolRegisterer{
	toolDefinition[AddParams, any]{
		Name:        "add",
		Description: "add two numbers, a and b",
		Handler:     Add,
	},
	toolDefinition[MultParams, any]{
		Name:        "mult",
		Description: "multiply two numbers, a and b",
		Handler:     Mult,
	},
	toolDefinition[LowerParams, any]{
		Name:        "lower",
		Description: "convert a string, s, to lowercase",
		Handler:     Lower,
	},
}

// TODO should we be able to specify the URI? ie /mcp
func RunTrivyServer(executableName string) http.Handler {
	server := mcp.NewServer(&mcp.Implementation{Name: executableName, Version: "v1.0.0"}, nil)

	for _, tool := range ToolsProvided {
		tool.Register(server)
	}

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	return handler
}
