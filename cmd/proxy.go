package cmd

import (
	"fmt"

	"grpc2mcp/internal/server"

	"github.com/spf13/cobra"
)

var (
	mcpHost string
	mcpPort int
	port    int
	mcpUri  string
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Starts the gRPC to MCP proxy",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Starting proxy to %s:%d%s on port %d\n", mcpHost, mcpPort, mcpUri, port)
		s, err := server.NewServer(mcpHost, mcpPort, mcpUri)
		if err != nil {
			return fmt.Errorf("failed to create proxy server: %w", err)
		}
		if err := s.Start(port); err != nil {
			return fmt.Errorf("failed to start proxy server: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(proxyCmd)
	proxyCmd.Flags().StringVar(&mcpHost, "mcp-host", "localhost", "The hostname of the MCP server")
	proxyCmd.Flags().IntVar(&mcpPort, "mcp-port", 8888, "The port of the MCP server")
	proxyCmd.Flags().StringVar(&mcpUri, "mcp-uri", "/mcp/", "The URI path for the MCP server")
	proxyCmd.Flags().IntVar(&port, "port", 8080, "The port for the proxy to listen on")
}
