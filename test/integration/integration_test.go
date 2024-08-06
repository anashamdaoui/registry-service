package integration

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"registry-service/internal/config"
	"registry-service/internal/middleware"
	"registry-service/internal/registry"
	"registry-service/internal/server"
	"testing"
	"time"

	"github.com/gorilla/mux"
)

// Initialize the logger for tests
func initTestLogger() {
	config.LoadConfig("config.json")
	middleware.InitLogger(config.AppConfig.LogLevel)
}

func setupServer(port string) (*registry.Registry, *mux.Router, *http.Server) {
	initTestLogger() // Initialize the logger
	reg := registry.NewRegistry()
	router := mux.NewRouter()
	ready := make(chan struct{})

	srv := server.StartServer(reg, router, ready, port)

	// Wait for the server to signal readiness
	<-ready
	log.Println("Server is ready.")
	time.Sleep(1 * time.Second) // Give the server a moment to be ready
	return reg, router, srv
}

func cleanupWorkersFile() {
	if err := os.Remove("workers.json"); err != nil && !os.IsNotExist(err) {
		log.Printf("Failed to remove workers.json: %v", err)
	} else {
		log.Println("workers.json file removed successfully.")
	}
}

func TestRegisterEndpoint(t *testing.T) {
	defer cleanupWorkersFile()
	log.Println("Setting up server for TestRegisterEndpoint...")
	reg, _, srv := setupServer("8081")
	defer srv.Close()

	address := "http://worker1:8080"
	url := "http://localhost:8081/register?address=" + address
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("X-API-Key", config.AppConfig.APIKey)

	log.Printf("Sending register request to URL: %s", url)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to send register request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200 OK, got %d", resp.StatusCode)
	}

	log.Println("Checking if the worker was registered correctly...")
	worker, exists := reg.GetWorker(address)
	if !exists {
		t.Fatalf("Expected worker to be registered")
	}

	if worker.Address != address {
		t.Errorf("Expected address %s, got %s", address, worker.Address)
	}
	log.Println("TestRegisterEndpoint completed successfully.")
}

func TestHealthEndpoint(t *testing.T) {
	defer cleanupWorkersFile()
	log.Println("Setting up server for TestHealthEndpoint...")
	reg, _, srv := setupServer("8082")
	defer srv.Close()

	// Start a mock HTTP server to simulate the worker
	mockWorker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Mock worker received request for: %s", r.URL.Path)
		if r.URL.Path == "/healthcheck" {
			log.Println("Mock worker responding with 200 OK")
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte("OK")); err != nil {
				log.Printf("Failed to write response: %v", err)
			}
		} else {
			log.Println("Mock worker responding with 404 Not Found")
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockWorker.Close()

	address := mockWorker.URL
	log.Printf("Registering worker with address: %s", address)
	reg.RegisterWorker(address)

	// Call CheckAllWorkers to update the health status of the worker
	reg.CheckAllWorkers()

	// Verify the worker's health status
	worker, exists := reg.GetWorker(address)
	if !exists {
		t.Fatalf("Expected worker to be found")
	}
	if !worker.IsHealthy {
		t.Fatalf("Expected worker to be healthy, got unhealthy")
	}
	if worker.Address != address {
		t.Fatalf("Expected worker address to be %s, got %s", address, worker.Address)
	}

	log.Println("TestHealthEndpoint completed successfully.")
}

func TestHealthyWorkersEndpoint(t *testing.T) {
	defer cleanupWorkersFile()
	log.Println("Setting up server for TestHealthyWorkersEndpoint...")
	reg, router, srv := setupServer("8083")
	defer srv.Close()

	address1 := "http://worker1:8080"
	address2 := "http://worker2:8080"

	log.Printf("Registering workers with addresses: %s and %s", address1, address2)
	reg.RegisterWorker(address1)
	reg.RegisterWorker(address2)
	reg.UpdateHealth(address2, false)

	url := "/workers/healthy"
	log.Printf("Sending request to URL: %s", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("X-API-Key", config.AppConfig.APIKey)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	log.Printf("Received response: %v", w.Code)
	log.Printf("Response body: %v", w.Body.String())

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK, got %d", w.Code)
	}

	var addresses []string
	if err := json.NewDecoder(w.Body).Decode(&addresses); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(addresses) != 1 || addresses[0] != address1 {
		t.Errorf("Expected healthy worker address %s, got %v", address1, addresses)
	}
	log.Println("TestHealthyWorkersEndpoint completed successfully.")
}
