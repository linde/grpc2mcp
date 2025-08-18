package cmd

import (
	"bytes"
	"io"
	"log"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

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
