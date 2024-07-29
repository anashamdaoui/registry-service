package main

import (
	"log"
	"os"
	"os/signal"
	"registry-service/internal/registry"
	"registry-service/internal/server"
	"syscall"

	"github.com/gorilla/mux"
)

func main() {
	log.Println("Starting registry service...")

	// Create a new registry
	reg := registry.NewRegistry()

	// Create a new router
	router := mux.NewRouter()

	// Channel to signal when the server is ready
	ready := make(chan struct{})

	// Define the port
	port := "8080"

	// Start the server in a separate goroutine
	srv := server.StartServer(reg, router, ready, port)

	// Wait for the server to signal readiness
	<-ready
	log.Println("Server is ready to handle requests.")

	// Set up signal handling for graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	<-sigs
	log.Println("Shutting down registry service...")

	if err := srv.Close(); err != nil {
		log.Fatalf("Server Shutdown Failed:%+v", err)
	}
	log.Println("Server exited properly")
}
