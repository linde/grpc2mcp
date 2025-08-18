package cmd

import (
	"context"
	"fmt"
	"grpc2mcp/internal/test"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var trivyCmd = &cobra.Command{
	Use:   "trivy",
	Short: "Runs the trivy test server",
	Long:  `Runs the trivy test server, which is an example MCP server.`,
	Run:   runTrivyServer,
}

var trivyName string
var trivyPort int

func init() {
	rootCmd.AddCommand(trivyCmd)
	trivyCmd.Flags().StringVarP(&trivyName, "name", "n", "trivy", "The name of the server")
	trivyCmd.Flags().IntVarP(&trivyPort, "port", "p", 8888, "The port to listen on")
}

func runTrivyServer(cmd *cobra.Command, args []string) {
	handler := test.RunTrivyServer(trivyName)

	addr := fmt.Sprintf(":%d", trivyPort)
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Trivy MCP server listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("could not listen on %s: %v\n", addr, err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exiting")
}
