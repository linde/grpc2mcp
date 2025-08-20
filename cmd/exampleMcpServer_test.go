package cmd

import (
	"testing"
	"time"
)

func TestExampleMCPServerCommand(t *testing.T) {

	rootCmd.SetArgs([]string{"exampleMcpServer"})
	runningCheckStr := []string{"Example MCP server 'mcp-go-server' listening on"}

	time.Sleep(100 * time.Millisecond)
	runSubCommand(t, rootCmd, 500*time.Millisecond, runningCheckStr)
}

func TestExampleMCPServerCommandWithFlags(t *testing.T) {

	rootCmd.SetArgs([]string{"exampleMcpServer", "--name=test-mcp-go", "--port=0"})
	runningCheckStr := []string{"Example MCP server 'test-mcp-go' listening on"}

	runSubCommand(t, rootCmd, 500*time.Millisecond, runningCheckStr)
}
