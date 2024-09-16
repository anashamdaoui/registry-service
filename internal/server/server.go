package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"registry-service/internal/middleware"
	"registry-service/internal/observability"
	"registry-service/internal/registry"
	"strings"

	"github.com/gorilla/mux"
)

func healthcheckHandler(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestIDFromContext(r.Context())
	logger := middleware.GetLogger()
	logger.Debug(requestID, "Handling /healthcheck request")

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("Healthy")); err != nil {
		logger.Debug(requestID, "Error writing response: %v", err)
	}
}

// StartServer starts the HTTP server for the registry service and returns the server instance.
func StartServer(reg *registry.Registry, router *mux.Router, ready chan struct{}, port string) *http.Server {

	router.Use(middleware.RequestID)            // Add Request ID middleware
	router.Use(middleware.LoggerMiddleware)     // Add Logger middleware
	router.Use(middleware.AuthMiddleware)       // Add Auth middleware
	router.Use(observability.MetricsMiddleware) // Add Metrics middleware

	router.HandleFunc("/healthcheck", healthcheckHandler).Methods("GET")

	router.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.GetRequestIDFromContext(r.Context())
		logger := middleware.GetLogger()
		logger.Debug(requestID, "Handling /register request")

		// Get the remote address (IP:Port) from the HTTP request
		remoteAddr := r.RemoteAddr

		// Extract just the IP address (without the port)
		ip := remoteAddr
		if colonIndex := strings.LastIndex(remoteAddr, ":"); colonIndex != -1 {
			ip = remoteAddr[:colonIndex] // Strip the port
		}

		// Read the request body to get the worker's port
		var requestData struct {
			ID   string `json:"id"`
			Port int    `json:"port"`
		}

		err := json.NewDecoder(r.Body).Decode(&requestData)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			logger.Debug(requestID, "Invalid request body")
			return
		}

		workerIP := fmt.Sprintf("%s:%d", ip, requestData.Port)
		logger.Debug(requestID, "Worker IP:Port = %s\n", workerIP)

		reg.RegisterWorker("http://" + workerIP)

		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("Worker registered")); err != nil {
			logger.Debug(requestID, "Error writing response: %v", err)
		}
	}).Methods("POST")

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

		addresses := reg.GetHealthyWorkersAddress()

		if err := json.NewEncoder(w).Encode(addresses); err != nil {
			logger.Debug(requestID, "Error encoding response: %v", err)
		}
	}).Methods("GET")

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	middleware.GetLogger().Info("", "Starting HTTP server on : %s...", port)
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
