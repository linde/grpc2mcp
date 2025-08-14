package cmd

import (
	"testing"
)

// This unit test executes the proxy command with default flags so we can debug
func TestProxyCommand(t *testing.T) {
	rootCmd.SetArgs([]string{"proxy"})

	rootCmd.Execute()

}
