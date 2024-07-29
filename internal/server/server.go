package server

import (
	"encoding/json"
	"log"
	"net/http"
	"registry-service/internal/registry"

	"github.com/gorilla/mux"
)

// StartServer starts the HTTP server for the registry service and returns the server instance.
func StartServer(reg *registry.Registry, router *mux.Router, ready chan struct{}, port string) *http.Server {
	log.Println("Configuring routes...")

	router.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		address := r.URL.Query().Get("address")
		if address == "" {
			http.Error(w, "Missing address", http.StatusBadRequest)
			return
		}
		log.Printf("Received register request for address: %s", address)
		reg.RegisterWorker(address)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Worker registered"))
	}).Methods("GET")

	router.HandleFunc("/worker/health/{address}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		address := vars["address"]
		log.Printf("Received health check request for address: %s", address)
		worker, found := reg.GetWorker(address)
		if !found {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(worker)
	}).Methods("GET")

	router.HandleFunc("/workers/healthy", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Received request for healthy workers")
		workers := reg.GetHealthyWorkers()
		addresses := make([]string, 0, len(workers))
		for _, worker := range workers {
			addresses = append(addresses, worker.Address)
		}
		json.NewEncoder(w).Encode(addresses)
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
	return srv
}
