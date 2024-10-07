package server

import (
	"encoding/json"
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

func registerHandler(w http.ResponseWriter, r *http.Request, reg *registry.Registry) {
	requestID := middleware.GetRequestIDFromContext(r.Context())
	logger := middleware.GetLogger()
	logger.Debug(requestID, "Handling /register request")

	remoteAddr := r.RemoteAddr
	ip := remoteAddr
	if colonIndex := strings.LastIndex(remoteAddr, ":"); colonIndex != -1 {
		ip = remoteAddr[:colonIndex]
	}

	var requestData struct {
		ID       string `json:"id"`
		HTTPPort int32  `json:"httpport"`
		GRPCPort int32  `json:"grpcport"`
	}

	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		logger.Debug(requestID, "Invalid request body")
		return
	}

	logger.Debug(requestID, "Worker ID : %s\n\tIP : %s\n\tHTTP Port : %d\n\tGRPC Port : %d\n", requestData.ID, ip, requestData.HTTPPort, requestData.GRPCPort)
	reg.RegisterWorker(requestData.ID, ip, requestData.HTTPPort, requestData.GRPCPort)

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("Worker registered")); err != nil {
		logger.Debug(requestID, "Error writing response: %v", err)
	}
}

type HealthResponse struct {
	HealthStatus string `json:"health_status"`
}

func workerHealthHandler(w http.ResponseWriter, r *http.Request, reg *registry.Registry) {
	requestID := middleware.GetRequestIDFromContext(r.Context())
	logger := middleware.GetLogger()

	url := r.URL.Query().Get("address")
	if url == "" {
		http.Error(w, "Missing address", http.StatusBadRequest)
		return
	}
	logger.Debug(requestID, "Handling /worker/health request for address : %s", url)

	bIsHealthy, found := reg.GetWorkerHealth(url)
	if !found {
		http.NotFound(w, r)
		return
	}

	health_status := "unhealthy"
	if bIsHealthy {
		health_status = "healthy"
	}
	data := HealthResponse{
		HealthStatus: health_status,
	}

	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Debug(requestID, "Error encoding response: %v", err)
	}
}

func healthyWorkersHandler(w http.ResponseWriter, r *http.Request, reg *registry.Registry) {
	requestID := middleware.GetRequestIDFromContext(r.Context())
	logger := middleware.GetLogger()
	logger.Debug(requestID, "Handling /workers/healthy request")

	addresses := reg.GetHealthyWorkersURL()
	if err := json.NewEncoder(w).Encode(addresses); err != nil {
		logger.Debug(requestID, "Error encoding response: %v", err)
	}
}

func setupRoutes(router *mux.Router, reg *registry.Registry) {
	router.HandleFunc("/healthcheck", healthcheckHandler).Methods("GET")
	router.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		registerHandler(w, r, reg)
	}).Methods("POST")
	router.HandleFunc("/worker/health", func(w http.ResponseWriter, r *http.Request) {
		workerHealthHandler(w, r, reg)
	}).Methods("GET")
	router.HandleFunc("/workers/healthy", func(w http.ResponseWriter, r *http.Request) {
		healthyWorkersHandler(w, r, reg)
	}).Methods("GET")
}

func setupMiddleware(router *mux.Router) {
	router.Use(middleware.RequestID)
	router.Use(middleware.LoggerMiddleware)
	router.Use(middleware.AuthMiddleware)
	router.Use(observability.MetricsMiddleware)
}

func startHTTPServer(srv *http.Server, ready chan struct{}) {
	log.Printf("Starting HTTP server on %s...", srv.Addr)
	close(ready)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server stopped: %v", err)
		}
	}()
}

// StartServer starts the HTTP server for the registry service and returns the server instance.
func StartServer(reg *registry.Registry, router *mux.Router, ready chan struct{}, port string) *http.Server {
	setupMiddleware(router)
	setupRoutes(router, reg)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	startHTTPServer(srv, ready)

	go observability.ServeMetrics(":9090")

	return srv
}
