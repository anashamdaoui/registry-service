package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"registry-service/internal/config"
	"registry-service/internal/database"
	"registry-service/internal/middleware"
	"registry-service/internal/registry"
	"registry-service/internal/server"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

// Test Setup:
// - setupIntegrationDB is used to initialize a clean database state before each test, ensuring that each test runs independently.
// - The init function ensures that configuration settings are loaded from the config.json file before tests are executed.
func init() {
	// Load the configuration from the config file
	config.LoadConfig("config.json")
	// Initialize the logger
	middleware.InitLogger(config.AppConfig.LogLevel)
}

// setupIntegrationDB initializes a clean database state for integration tests.
func setupIntegrationDB(t *testing.T) *database.MongoDB {
	// Connect to the MongoDB instance
	db, err := database.NewMongoDB(config.AppConfig.DB.URI, config.AppConfig.DB.Name, config.AppConfig.DB.Collection)
	assert.NoError(t, err, "Failed to connect to test database")

	// Clear the existing collection to start fresh
	err = db.ClearCollection()
	assert.NoError(t, err, "Failed to clean up test database")

	return db
}

// setupTestServer starts a test HTTP server for the registry service.
func setupTestServer(db *database.MongoDB) (*httptest.Server, *registry.Registry) {
	checkInterval := time.Duration(config.AppConfig.CheckIntervalMs) * time.Millisecond
	reg := registry.NewRegistry(db, checkInterval)

	router := mux.NewRouter()
	server.StartServer(reg, router, make(chan struct{}), "")

	return httptest.NewServer(router), reg
}

// Send a POST /register to the registry server with worker port in the body
func registerWorker(registryURL string, id string, port int, apiKey string) (*http.Response, error) {
	workerData := map[string]interface{}{
		"id":   id,
		"port": port,
	}

	jsonData, _ := json.Marshal(workerData)

	req, err := http.NewRequest("POST", registryURL+"/register", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	// Add the required headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	return resp, nil
}

// TestIntegrationRegisterWorker tests the registration of a worker through the HTTP API.
func TestIntegrationRegisterWorker(t *testing.T) {
	db := setupIntegrationDB(t)
	defer db.Disconnect()

	ts, _ := setupTestServer(db)
	defer ts.Close()

	address := "http://127.0.0.1:1234"
	resp, err := registerWorker(ts.URL, "1-2-3-4", 1234, config.AppConfig.APIKey)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify worker is registered in the database
	workers, err := db.GetAllWorkers()
	assert.NoError(t, err)
	assert.Len(t, workers, 1)
	assert.Equal(t, address, workers[0]["address"])
}

// TestIntegrationGetWorkerHealth tests retrieving the health status of a worker.
func TestIntegrationGetWorkerHealth(t *testing.T) {
	db := setupIntegrationDB(t)
	defer db.Disconnect()

	ts, reg := setupTestServer(db)
	defer ts.Close()

	address := "http://worker.domain:1234"
	_ = db.InsertWorker(address)
	reg.RegisterWorker(address) // register the server in the registry cache

	req, err := http.NewRequest("GET", ts.URL+"/worker/health?address="+address, nil)
	assert.NoError(t, err)

	// Include API Key in the request header
	req.Header.Set("X-API-Key", config.AppConfig.APIKey)

	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var worker registry.Worker
	err = json.NewDecoder(resp.Body).Decode(&worker)
	assert.NoError(t, err)
	assert.Equal(t, address, worker.Address)
}

// TestIntegrationGetHealthyWorkers tests retrieving all healthy workers.
func TestIntegrationGetHealthyWorkers(t *testing.T) {
	db := setupIntegrationDB(t)
	defer db.Disconnect()

	ts, reg := setupTestServer(db)
	defer ts.Close()

	address1 := "http://worker3.1:8080"
	_ = db.InsertWorker(address1)
	reg.RegisterWorker(address1) // register the server in the registry cache

	address2 := "http://worker3.2:8080"
	_ = db.InsertWorker(address2)
	reg.RegisterWorker(address2) // register the server in the registry cache

	// Fake out a worker healthcheck failure
	db.UpdateWorkerHealth(address2, false)
	reg.UpdateHealth(address2, false)

	req, err := http.NewRequest("GET", ts.URL+"/workers/healthy", nil)
	assert.NoError(t, err)

	// Include API Key in the request header
	req.Header.Set("X-API-Key", config.AppConfig.APIKey)

	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var addresses []string
	err = json.NewDecoder(resp.Body).Decode(&addresses)
	assert.NoError(t, err)
	assert.Len(t, addresses, 1) // Only one worker should be healthy
	assert.Equal(t, address1, addresses[0])
}

// TestIntegrationHealthCheckLoop verifies that the health check loop updates worker health.
func TestIntegrationHealthCheckLoop(t *testing.T) {
	db := setupIntegrationDB(t)
	defer db.Disconnect()

	ts, reg := setupTestServer(db)
	defer ts.Close()

	// Start a mock server to simulate the worker
	mockWorker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthcheck" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockWorker.Close()

	address := mockWorker.URL
	reg.RegisterWorker(address) // register the server in the registry cache

	// Wait for the health check loop to run
	time.Sleep(2 * time.Second)

	// Verify the worker is marked as healthy
	req, err := http.NewRequest("GET", ts.URL+"/worker/health?address="+address, nil)
	assert.NoError(t, err)

	// Include API Key in the request header
	req.Header.Set("X-API-Key", config.AppConfig.APIKey)

	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var worker registry.Worker
	err = json.NewDecoder(resp.Body).Decode(&worker)
	assert.NoError(t, err)
	assert.True(t, worker.IsHealthy, "Worker should be healthy after health check loop")
}
