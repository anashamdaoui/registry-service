package integration

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"registry-service/internal/registry"
	"registry-service/internal/server"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
)

func setupServer(port string) (*registry.Registry, *mux.Router, *http.Server) {
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

func TestRegisterEndpoint(t *testing.T) {
	log.Println("Setting up server for TestRegisterEndpoint...")
	reg, _, srv := setupServer("8081")
	defer srv.Close()

	address := "http://worker1:8080"
	url := "http://localhost:8081/register?address=" + address
	log.Printf("Sending register request to URL: %s", url)
	resp, err := http.Get(url)
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
	log.Println("Setting up server for TestHealthEndpoint...")
	reg, router, srv := setupServer("8082")
	defer srv.Close()

	address := "http://worker1:8080"
	log.Printf("Registering worker with address: %s", address)
	reg.RegisterWorker(address)

	// Strip the protocol from the address for the path variable
	addressPath := strings.TrimPrefix(address, "http://")
	url := "/worker/health/" + addressPath
	log.Printf("Sending health check request to URL: %s", url)
	req := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	log.Printf("Received response: %v", w.Code)
	log.Printf("Response body: %v", w.Body.String())

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK, got %d", w.Code)
	}

	var worker registry.Worker
	err := json.NewDecoder(w.Body).Decode(&worker)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if worker.Address != address {
		t.Errorf("Expected address %s, got %s", address, worker.Address)
	}
	log.Println("TestHealthEndpoint completed successfully.")
}

func TestHealthyWorkersEndpoint(t *testing.T) {
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
	req := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	log.Printf("Received response: %v", w.Code)
	log.Printf("Response body: %v", w.Body.String())

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK, got %d", w.Code)
	}

	var addresses []string
	err := json.NewDecoder(w.Body).Decode(&addresses)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(addresses) != 1 || addresses[0] != address1 {
		t.Errorf("Expected healthy worker address %s, got %v", address1, addresses)
	}
	log.Println("TestHealthyWorkersEndpoint completed successfully.")
}
