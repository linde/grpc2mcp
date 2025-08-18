package cmd

import (
	"testing"
	"time"
)

func TestExampleMCPCommand(t *testing.T) {

	rootCmd.SetArgs([]string{"exampleMCP"})
	runningCheckStr := []string{"Example MCP server 'trivy' listening on"}

	runSubCommand(t, rootCmd, 250*time.Millisecond, runningCheckStr)
}
