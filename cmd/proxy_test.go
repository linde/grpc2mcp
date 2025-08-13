package cmd

import (
	"strings"
	"testing"
)

func TestProxyCommand_Unit(t *testing.T) {
	// This unit test executes the proxy command with default flags.
	// It expects an error because the downstream MCP server is not running.
	// The test passes if the command fails with the expected error message,
	// which proves the command's logic is being executed.

	rootCmd.SetArgs([]string{"proxy"})

	err := rootCmd.Execute()

	if err == nil {
		t.Fatal("expected an error, but got none")
	}

	// Check for the specific error that occurs when the downstream server is unavailable.
	// This confirms that the command ran and tried to initialize the server.
	expectedErr := "failed to initialize MCP session"
	if !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("expected error to contain %q, but got: %v", expectedErr, err)
	}
}
