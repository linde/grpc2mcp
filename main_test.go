package main

import (
	"bytes"
	"grpc2mcp/cmd"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestHelpCommandSmoke tests that the --help command executes without errors
// and produces the expected output.
func TestHelpCommandSmoke(t *testing.T) {
	// Cobra commands write to os.Stdout and os.Stderr by default.
	// To test them, we can redirect them to a buffer.
	
	// Keep track of the original os.Args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Set os.Args to simulate running the '--help' command.
	os.Args = []string{"grpc2mcp", "--help"}

	// The root command object is not directly exposed, but Execute() uses it.
	// We can't easily prevent the os.Exit call that help commands often trigger.
	// However, for a smoke test, we can simply execute the command and
	// trust the test runner to fail if a panic occurs.
	// A more advanced test would involve refactoring cmd.Execute to allow for injection.
	
	// To capture output, we can temporarily replace os.Stdout
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w
	
	cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	
	// Assert that the output contains the usage string, which indicates
	// the help text was printed successfully.
	assert.Contains(t, buf.String(), "Usage:")
	assert.Contains(t, buf.String(), "proxy") // Check for subcommand
}
