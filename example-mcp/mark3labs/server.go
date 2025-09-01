package main

import (
	"context"
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Create a new MCP server
	s := server.NewMCPServer(
		"Demo 🚀",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true),
		server.WithPromptCapabilities(true),
	)

	// Add tool

	addTool := mcp.NewTool("add",
		mcp.WithDescription("Add two numbers"),
		mcp.WithNumber("a",
			mcp.Required(),
			mcp.Description("The first number to be added"),
		),
		mcp.WithNumber("b",
			mcp.Required(),
			mcp.Description("The other number to be added"),
		))

	s.AddTool(addTool, addHandler)

	port := 8888
	uri := "/mcp"

	httpServer := server.NewStreamableHTTPServer(s, server.WithEndpointPath(uri))
	defer httpServer.Shutdown(context.Background())

	addr := fmt.Sprintf(":%d", port)
	log.Printf("Example MCP server listening on %s%s", addr, uri)
	if err := httpServer.Start(addr); err != nil {
		log.Fatalf("could not listen on %s: %v\n", addr, err)
	}

}

func addHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := request.RequireFloat("a")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	b, err := request.RequireFloat("b")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("%v", a+b)), nil
}
