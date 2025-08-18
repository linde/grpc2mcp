package cmd

import (
	"fmt"
	"grpc2mcp/internal/proxy"
	"log"

	"github.com/spf13/cobra"
)

var (
	mcpHost string
	mcpPort int
	mcpUri  string
	port    int
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Starts the gRPC to MCP proxy",
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Printf("starting proxy to %s:%d%s on port %d\n", mcpHost, mcpPort, mcpUri, port)
		s, err := proxy.NewServer(mcpHost, mcpPort, mcpUri)
		if err != nil {
			return fmt.Errorf("failed to create proxy server: %w", err)
		}

		lisAddr, shutdownFunc, err := s.StartAsync(port)
		defer shutdownFunc()
		if err != nil {
			return fmt.Errorf("failed to start proxy server: %w", err)
		}
		log.Printf("proxy server listening on: %v:%v", lisAddr.IP, lisAddr.Port)

		select {} // wait
	},
}

func init() {
	rootCmd.AddCommand(proxyCmd)
	proxyCmd.Flags().StringVar(&mcpHost, "mcp-host", "localhost", "The hostname of the MCP server")
	proxyCmd.Flags().IntVar(&mcpPort, "mcp-port", 8888, "The port of the MCP server")
	proxyCmd.Flags().StringVar(&mcpUri, "mcp-uri", "/mcp/", "The URI path for the MCP server")
	proxyCmd.Flags().IntVar(&port, "port", 8080, "The port for the proxy to listen on")
}
