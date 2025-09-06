package cmd

import (
	"context"
	"fmt"
	"grpc2mcp/internal/examplemcp"
	"log"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

var exampleMCPCmd = &cobra.Command{
	Use:   "example-mcp",
	Short: "Runs an example MCP server",
	Run:   runExampleMCPServer,
}

var exampleMCPName string
var exampleMCPPort int

func init() {
	rootCmd.AddCommand(exampleMCPCmd)
	exampleMCPCmd.Flags().StringVarP(&exampleMCPName, "name", "n", "trivy", "The name of the server")
	exampleMCPCmd.Flags().IntVarP(&exampleMCPPort, "port", "p", 8888, "The port to listen on")
}

func runExampleMCPServer(cmd *cobra.Command, args []string) {
	handler := examplemcp.RunTrivyServer(exampleMCPName)

	addr := fmt.Sprintf(":%d", exampleMCPPort)
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Example MCP server '%s' listening on %s", exampleMCPName, addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("could not listen on %s: %v\n", addr, err)
		}
	}()

	// Wait for the context to be cancelled
	<-cmd.Context().Done()
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server shutdown timeout exceeded and it was forced to shutdown:", err)
	}

	log.Println("Server exiting")
}
