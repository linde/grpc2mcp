package examplemcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type ProvidedTool struct {
	tool    mcp.Tool
	handler server.ToolHandlerFunc
}

func (pt ProvidedTool) GetName() string {
	return pt.tool.Name
}

// TODO consider inlining this type if it isnt used outside the package
var ToolsProvided = []ProvidedTool{
	{
		mcp.NewTool("add",
			mcp.WithDescription("Add two numbers"),
			mcp.WithNumber("a", mcp.Required()),
			mcp.WithNumber("b", mcp.Required()),
		), doMath,
	},
	{
		mcp.NewTool("mult",
			mcp.WithDescription("Mulitply two numbers"),
			mcp.WithNumber("a", mcp.Required()),
			mcp.WithNumber("b", mcp.Required()),
		), doMath,
	},
	{
		mcp.NewTool("lower",
			mcp.WithDescription("lower case a string"),
			mcp.WithNumber("l", mcp.Required()),
		), doLower,
	},
}

type ProvidedPrompt struct {
	prompt  mcp.Prompt
	handler server.PromptHandlerFunc
}

func (pp ProvidedPrompt) GetName() string {
	return pp.prompt.Name
}

var PromptsProvided = []ProvidedPrompt{
	{
		mcp.NewPrompt("greet",
			mcp.WithPromptDescription("A prompt that greets the user"),
			mcp.WithArgument("whom", mcp.ArgumentDescription("whom gets greeted"), mcp.RequiredArgument()),
		), doGreetPrompt,
	},
}

func RunExampleMcpServer(serverName string, uri string) http.Handler {
	s := server.NewMCPServer(serverName,
		"0.0.0",
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true),
		server.WithPromptCapabilities(true),
	)

	// Add the tools/prompts
	for _, tp := range ToolsProvided {
		s.AddTool(tp.tool, tp.handler)
	}

	for _, pp := range PromptsProvided {
		s.AddPrompt(pp.prompt, pp.handler)
	}

	// TODO consider having an optional param for uri that defaults to /mcp
	httpServer := server.NewStreamableHTTPServer(s, server.WithEndpointPath(uri))

	return httpServer
}

// below are the handlers for the respective MCP entities

func doMath(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {

	a, err := request.RequireFloat("a")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	b, err := request.RequireFloat("b")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	switch request.Params.Name {
	case "add":
		return mcp.NewToolResultText(fmt.Sprintf("%v", a+b)), nil
	case "mult":
		return mcp.NewToolResultText(fmt.Sprintf("%v", a*b)), nil
	}

	return nil, fmt.Errorf("doMath called with unsupported tool name")
}

func doLower(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {

	s, err := request.RequireString("s")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(strings.ToLower(s)), nil
}

// TODO get something more idiomatic. this is just a call repsonse
func doGreetPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {

	args := request.Params.Arguments
	return &mcp.GetPromptResult{
		Description: "A simple prompt to greet someone",
		Messages: []mcp.PromptMessage{
			{
				Role: mcp.RoleUser,
				Content: mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("What's up, %s?", args["whom"]),
				},
			},
		},
	}, nil
}
