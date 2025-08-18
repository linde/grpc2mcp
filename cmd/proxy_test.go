package cmd

import (
	"testing"
	"time"
)

// This unit test executes the proxy command with default flags so we can debug
func TestProxyCommand(t *testing.T) {

	rootCmd.SetArgs([]string{"proxy"})
	runningCheckStr := []string{"proxy server listening on"}

	runSubCommand(t, rootCmd, 250*time.Millisecond, runningCheckStr)

}
