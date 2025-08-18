package cmd

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestExampleMCPCommand(t *testing.T) {

	// Create a cancellable context
	cancelableCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run the command in a goroutine using GenericCommandRunner to capture output
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		rootCmd.SetArgs([]string{"exampleMCP"})
		rootCmd.SetContext(cancelableCtx)
		runningCheckStr := "Example MCP server 'trivy' listening on"
		GenericCommandRunner(t, rootCmd, runningCheckStr)
	}()

	// we need to wait for the command to start ...
	time.Sleep(250 * time.Millisecond)
	// ... then cancel it
	cancel()
	// don't exit until it has called our wg.Done()
	wg.Wait()

}
