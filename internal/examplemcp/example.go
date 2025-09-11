package examplemcp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	PARAM_WHOM = "whom"
	PARAM_A    = "a"
	PARAM_B    = "b"
	PARAM_S    = "s"

	TOOL_ADD            = "add"
	TOOL_MULT           = "mult"
	TOOL_LOWER          = "lower"
	TOOL_GREET_RESOURCE = "greetResource"

	RESOURCE_URI_STATIC = "test://static/resource"

	PROMPT_GREET = "greet"
)

func GetProvidedToolNames() []string {

	names := make([]string, len(toolsProvided))

	for idx, providedTool := range toolsProvided {
		names[idx] = providedTool.tool.GetName()
	}
	sort.Strings(names)
	return names
}

var toolsProvided = []struct {
	tool    mcp.Tool
	handler server.ToolHandlerFunc
}{
	{
		mcp.NewTool(TOOL_ADD,
			mcp.WithDescription("Add two numbers"),
			mcp.WithNumber(PARAM_A, mcp.Required()),
			mcp.WithNumber(PARAM_B, mcp.Required()),
		), doMath,
	},
	{
		mcp.NewTool(TOOL_MULT,
			mcp.WithDescription("Mulitply two numbers"),
			mcp.WithNumber(PARAM_A, mcp.Required()),
			mcp.WithNumber(PARAM_B, mcp.Required()),
		), doMath,
	},
	{
		mcp.NewTool(TOOL_LOWER,
			mcp.WithDescription("lower case a string"),
			mcp.WithString(PARAM_S, mcp.Required()),
		), doLower,
	},
	{
		mcp.NewTool(TOOL_GREET_RESOURCE,
			mcp.WithDescription("example to get a greeting via a resource"),
			mcp.WithString(PARAM_WHOM, mcp.Required()),
		), doToolWithResourceLink,
	},
}

func GetProvidedPrompts() []string {

	names := make([]string, len(promptsProvided))

	for idx, providedTool := range promptsProvided {
		names[idx] = providedTool.prompt.GetName()
	}
	sort.Strings(names)
	return names
}

var promptsProvided = []struct {
	prompt  mcp.Prompt
	handler server.PromptHandlerFunc
}{
	{
		mcp.NewPrompt(
			PROMPT_GREET,
			mcp.WithPromptDescription("A prompt that greets the user"),
			mcp.WithArgument(PARAM_WHOM, mcp.ArgumentDescription("whom gets greeted"), mcp.RequiredArgument()),
		), doGreetPrompt,
	},
}

var ResourcesProvided = []mcp.Resource{
	mcp.NewResource(RESOURCE_URI_STATIC, "Static Resource", mcp.WithMIMEType("text/plain")),
}

func RunExampleMcpServer(serverName string, uri string) http.Handler {
	s := server.NewMCPServer(serverName,
		"0.0.0",
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true),
		server.WithPromptCapabilities(true),
	)

	// Add the tools/prompts
	for _, tp := range toolsProvided {
		s.AddTool(tp.tool, tp.handler)
	}

	for _, pp := range promptsProvided {
		s.AddPrompt(pp.prompt, pp.handler)
	}

	for _, rp := range ResourcesProvided {
		s.AddResource(rp, handleReadResource)
	}

	// TODO consider having an optional param for uri that defaults to /mcp
	httpServer := server.NewStreamableHTTPServer(s, server.WithEndpointPath(uri))

	return httpServer
}

// below are the handlers for the respective MCP entities

func doMath(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {

	a, err := request.RequireFloat(PARAM_A)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	b, err := request.RequireFloat(PARAM_B)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	switch request.Params.Name {
	case TOOL_ADD:
		return mcp.NewToolResultText(fmt.Sprintf("%v", a+b)), nil
	case TOOL_MULT:
		return mcp.NewToolResultText(fmt.Sprintf("%v", a*b)), nil
	}

	return nil, fmt.Errorf("doMath called with unsupported tool name")
}

func doLower(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {

	s, err := request.RequireString(PARAM_S)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(strings.ToLower(s)), nil
}

func doToolWithResourceLink(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {

	whoParam, err := request.RequireString(PARAM_WHOM)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	escapedGreetingStr := url.QueryEscape(fmt.Sprintf("Hello, %s!", whoParam))
	mimeType := "text/plain"
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewResourceLink(
				fmt.Sprintf("data:%s", escapedGreetingStr),
				fmt.Sprintf("Sample %s", request.Method),
				fmt.Sprintf("A sample %s for demonstration", request.Method),
				mimeType,
			),
		},
	}, nil
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

func handleReadResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "test://static/resource",
			MIMEType: "text/plain",
			Text:     "This is a sample resource",
		},
	}, nil
}
