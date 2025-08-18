package cmd

import (
	"bytes"
	"context"
	"io"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// TODO make it so we dont have to pass the testing.T into here so maybe more portable?
func GenericCommandRunner(t *testing.T, cmd *cobra.Command, outputAssertions ...string) string {
	assert := assert.New(t)
	assert.NotNil(cmd)

	b := bytes.NewBufferString("")
	log.SetOutput(b)
	cmd.SetOut(b)
	cmd.SetErr(b)

	cmd.Execute()
	out, _ := io.ReadAll(b)

	for _, oa := range outputAssertions {
		assert.Contains(strings.ToLower(string(out)), strings.ToLower(oa))
	}
	return string(out)
}

func runSubCommand(t *testing.T, cmd *cobra.Command, wait time.Duration, outputAssertions []string) {

	// Create a cancellable context
	cancelableCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run the command in a goroutine using GenericCommandRunner to capture output
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		rootCmd.SetContext(cancelableCtx)
		GenericCommandRunner(t, rootCmd, outputAssertions...)
	}()

	// we need to wait for the command to start ...
	time.Sleep(wait)
	// ... then cancel it
	cancel()
	// don't exit until it has called our wg.Done()
	wg.Wait()
}
