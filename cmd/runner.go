package cmd

import (
	"bytes"
	"io"
	"log"

	"github.com/spf13/cobra"
)

func CommandRunner(cmd *cobra.Command) (string, error) {

	b := bytes.NewBufferString("")
	log.SetOutput(b)
	cmd.SetOut(b)
	cmd.SetErr(b)

	err := cmd.Execute()
	if err != nil {
		return "", err
	}
	out, err := io.ReadAll(b)

	return string(out), err
}
