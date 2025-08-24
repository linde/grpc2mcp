package cmd

import (
	"fmt"
	"grpc2mcp/internal/proxy"
	"log"

	"github.com/spf13/cobra"
)

var (
	mcpUrl string
	port   int
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Starts the gRPC to MCP proxy",
	RunE:  doProxy,
}

func doProxy(cmd *cobra.Command, args []string) error {

	log.Printf("starting proxy to %s on port %d\n", mcpUrl, port)
	s, err := proxy.NewServer(mcpUrl)
	if err != nil {
		return fmt.Errorf("failed to create proxy server: %w", err)
	}

	lisAddr, shutdownFunc, err := s.StartAsync(port)
	defer shutdownFunc()
	if err != nil {
		return fmt.Errorf("failed to start proxy server: %w", err)
	}
	log.Printf("proxy server listening on: %v:%v", lisAddr.IP, lisAddr.Port)

	// Wait for the context to be cancelled
	<-cmd.Context().Done()
	log.Println("Shutting down proxy server...")

	// shutdownFunc is called from defer above
	return nil
}

func init() {
	rootCmd.AddCommand(proxyCmd)
	proxyCmd.Flags().StringVar(&mcpUrl, "mcp-url", "http://localhost:8888/mcp/", "The http/https URL of the MCP server")
	proxyCmd.Flags().IntVar(&port, "port", 8080, "The port for the proxy to listen on")
}
