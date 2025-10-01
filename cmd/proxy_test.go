package cmd

import (
	"testing"
	"time"
)

// This unit test executes the proxy command with default flags so we can debug
func TestProxyCommand(t *testing.T) {

	rootCmd.SetArgs([]string{"proxy", "--port=0"}) //, "--mcp-url=https://api.githubcopilot.com/mcp/"})
	runningCheckStr := []string{"proxy server listening on"}

	runSubCommand(t, rootCmd, 250*time.Millisecond, runningCheckStr)
	// runSubCommand(t, rootCmd, 250*time.Hour, runningCheckStr)

}

func TestRunningProxyCommand(t *testing.T) {

	rootCmd.SetArgs([]string{"proxy", "--port=8080", "--mcp-url=https://api.githubcopilot.com/mcp/"})
	runningCheckStr := []string{"proxy server listening on"}

	// runSubCommand(t, rootCmd, 250*time.Millisecond, runningCheckStr)
	runSubCommand(t, rootCmd, 250*time.Hour, runningCheckStr)

}
