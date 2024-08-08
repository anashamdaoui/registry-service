package main

import (
	"log"
	"os"
	"os/signal"
	"registry-service/internal/config"
	"registry-service/internal/database"
	"registry-service/internal/middleware"
	"registry-service/internal/registry"
	"registry-service/internal/server"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

func main() {
	log.Println("Starting registry service...")

	// Load configuration
	config.LoadConfig("internal/config/config.json")

	// Initialize the logger with the configured log level
	middleware.InitLogger(config.AppConfig.LogLevel)

	// Connect to the MongoDB database
	db, err := database.NewMongoDB(config.AppConfig.DB.URI, config.AppConfig.DB.Name, config.AppConfig.DB.Collection)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer func() {
		if err := db.Disconnect(); err != nil {
			log.Fatalf("Error disconnecting from database: %v", err)
		}
	}() // Ensure the database connection is closed

	// Ensure indexes are created
	if err := db.CreateIndexes(); err != nil {
		log.Fatalf("Failed to create indexes: %v", err)
	}

	// Create a new registry
	checkInterval := time.Millisecond * time.Duration(config.AppConfig.CheckIntervalMs)
	reg := registry.NewRegistry(db, checkInterval)

	// Create a new router
	router := mux.NewRouter()

	// Channel to signal when the server is ready
	ready := make(chan struct{})

	// Start the server in a separate goroutine
	srv := server.StartServer(reg, router, ready, config.AppConfig.ServerPort)

	// Wait for the server to signal readiness
	<-ready
	log.Println("Server is ready to handle requests.")

	// Set up signal handling for graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	<-sigs
	log.Println("Shutting down registry service...")

	// Stop the health check loop
	reg.StopHealthCheck()

	if err := srv.Close(); err != nil {
		log.Fatalf("Server Shutdown Failed: %+v", err)
	}
	log.Println("Server exited properly")
}
