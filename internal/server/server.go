package server

import (
	"encoding/json"
	"log"
	"net/http"
	"registry-service/internal/middleware"
	"registry-service/internal/observability"
	"registry-service/internal/registry"

	"github.com/gorilla/mux"
)

// StartServer starts the HTTP server for the registry service and returns the server instance.
func StartServer(reg *registry.Registry, router *mux.Router, ready chan struct{}, port string) *http.Server {

	router.Use(middleware.RequestID)            // Add Request ID middleware
	router.Use(middleware.LoggerMiddleware)     // Add Logger middleware
	router.Use(middleware.AuthMiddleware)       // Add Auth middleware
	router.Use(observability.MetricsMiddleware) // Add Metrics middleware

	router.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.GetRequestIDFromContext(r.Context())
		logger := middleware.GetLogger()
		logger.Debug(requestID, "Handling /register request")

		address := r.URL.Query().Get("address")
		if address == "" {
			http.Error(w, "Missing address", http.StatusBadRequest)
			return
		}
		logger.Debug(requestID, "Received register request for address: %s", address)
		reg.RegisterWorker(address)
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("Worker registered")); err != nil {
			logger.Debug(requestID, "Error writing response: %v", err)
		}
	}).Methods("GET")

	router.HandleFunc("/worker/health", func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.GetRequestIDFromContext(r.Context())
		logger := middleware.GetLogger()
		logger.Debug(requestID, "Handling /worker/health request")

		address := r.URL.Query().Get("address")
		if address == "" {
			http.Error(w, "Missing address", http.StatusBadRequest)
			return
		}
		logger.Debug(requestID, "Received health check request for address: %s", address)

		worker, found := reg.GetWorker(address)
		if !found {
			http.NotFound(w, r)
			return
		}
		if err := json.NewEncoder(w).Encode(worker); err != nil {
			logger.Debug(requestID, "Error encoding response: %v", err)
		}
	}).Methods("GET")

	router.HandleFunc("/workers/healthy", func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.GetRequestIDFromContext(r.Context())
		logger := middleware.GetLogger()
		logger.Debug(requestID, "Handling /workers/healthy request")

		logger.Debug(requestID, "Received request for healthy workers")
		workers := reg.GetHealthyWorkers()
		addresses := make([]string, 0, len(workers))
		for _, worker := range workers {
			addresses = append(addresses, worker.Address)
		}
		if err := json.NewEncoder(w).Encode(addresses); err != nil {
			logger.Debug(requestID, "Error encoding response: %v", err)
		}
	}).Methods("GET")

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	log.Printf("Starting HTTP server on :%s...", port)
	close(ready) // Signal that the server is ready
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server stopped: %v", err)
		}
	}()

	// Start a separate HTTP server for Prometheus metrics
	go observability.ServeMetrics(":9090")

	return srv
}
