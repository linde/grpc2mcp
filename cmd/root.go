package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "grpc2mcp",
	Short: "A gRPC to MCP proxy",
	Long:  `A gRPC to MCP proxy that listens for gRPC calls and forwards them to an MCP server.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
