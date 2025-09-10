package cmd

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runSubCommand(t *testing.T, cmd *cobra.Command, wait time.Duration, outputAssertions []string) {

	assert := assert.New(t)

	// Create a cancellable context
	cancelableCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run the command in a goroutine using CommandRunner to capture output
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		cmd.SetContext(cancelableCtx)
		commandOutputStr, err := CommandRunner(cmd)
		require.NoError(t, err)

		lowerCommandOutputStr := strings.ToLower(commandOutputStr)
		for _, oa := range outputAssertions {
			assert.Contains(lowerCommandOutputStr, strings.ToLower(oa))
		}
	}()

	// we need to wait for the command to start ...
	time.Sleep(wait)
	// ... then cancel it
	cancel()
	// don't exit until it has called our wg.Done()
	wg.Wait()
}
