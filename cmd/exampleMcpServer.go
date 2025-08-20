package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

var exampleMcpServerCmd = &cobra.Command{
	Use:   "exampleMcpServer",
	Short: "Runs an example MCP server using mcp-go",
	Long:  `Runs an example MCP server using mcp-go`,
	Run:   runExampleMCPServerWithMcpGo,
}

var exampleMcpServerName string
var exampleMcpServerPort int

func init() {
	rootCmd.AddCommand(exampleMcpServerCmd)
	exampleMcpServerCmd.Flags().StringVarP(&exampleMcpServerName, "name", "n", "mcp-go-server", "The name of the server")
	exampleMcpServerCmd.Flags().IntVarP(&exampleMcpServerPort, "port", "p", 8889, "The port to listen on")
}

func runExampleMCPServerWithMcpGo(cmd *cobra.Command, args []string) {

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

	mcpServer := server.NewMCPServer(exampleMcpServerName, "1.0.0")
	mcpServer.AddTool(addTool, addHandler)

	httpServer := server.NewStreamableHTTPServer(mcpServer)
	defer httpServer.Shutdown(context.Background())

	addr := fmt.Sprintf(":%d", exampleMcpServerPort)
	log.Printf("Example MCP server '%s' listening on %s", exampleMcpServerName, addr)
	if err := httpServer.Start(addr); err != nil {
		log.Fatalf("could not listen on %s: %v\n", addr, err)
	}
}

func addHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	a, err := request.RequireString("a")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	b, err := request.RequireString("b")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("%v", a+b)), nil
}
