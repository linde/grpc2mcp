package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var portVar = flag.Int("port", 8888, "port to use for http transport")

type AddParams struct {
	A int `json:"a" jsonschema:"the first number"`
	B int `json:"b" jsonschema:"the second number"`
}

func Add(ctx context.Context, req *mcp.ServerRequest[*mcp.CallToolParamsFor[AddParams]]) (*mcp.CallToolResultFor[any], error) {
	result := req.Params.Arguments.A + req.Params.Arguments.B
	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{&mcp.TextContent{Text: strconv.Itoa(result)}},
	}, nil
}

type MultParams struct {
	A int `json:"a" jsonschema:"the first number"`
	B int `json:"b" jsonschema:"the second number"`
}

func Mult(ctx context.Context, req *mcp.ServerRequest[*mcp.CallToolParamsFor[MultParams]]) (*mcp.CallToolResultFor[any], error) {
	result := req.Params.Arguments.A * req.Params.Arguments.B
	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{&mcp.TextContent{Text: strconv.Itoa(result)}},
	}, nil
}

type LowerParams struct {
	S string `json:"s" jsonschema:"the string to convert to lowercase"`
}

func Lower(ctx context.Context, req *mcp.ServerRequest[*mcp.CallToolParamsFor[LowerParams]]) (*mcp.CallToolResultFor[any], error) {
	result := strings.ToLower(req.Params.Arguments.S)
	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{&mcp.TextContent{Text: result}},
	}, nil
}

func main() {
	flag.Parse()
	executableName := filepath.Base(os.Args[0])
	server := mcp.NewServer(&mcp.Implementation{Name: executableName, Version: "v1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "add", Description: "add two numbers"}, Add)
	mcp.AddTool(server, &mcp.Tool{Name: "mult", Description: "multiply two numbers"}, Mult)
	mcp.AddTool(server, &mcp.Tool{Name: "lower", Description: "convert a string to lowercase"}, Lower)

	httpAddr := fmt.Sprintf(":%d", *portVar)

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	log.Printf("MCP handler listening at %s", httpAddr)
	if err := http.ListenAndServe(httpAddr, handler); err != nil {
		log.Fatal(err)
	}

}
