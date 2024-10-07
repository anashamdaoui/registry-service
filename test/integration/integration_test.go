package integration

import (
	"bytes"
	"encoding/json"
	"log"
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
// - The init function ensures that configuration settings are loaded from the config.json file before tests are executed.
func init() {
	// Load the configuration from the config file
	config.LoadConfig("config.json")
	// Initialize the logger
	middleware.InitLogger(config.AppConfig.LogLevel)
}

// - setupIntegrationDB initializes a clean database state for integration tests.
func setupIntegrationDB(t *testing.T) *database.MongoDB {
	// Connect to the MongoDB instance
	db, err := database.NewMongoDB(config.AppConfig.DB.URI, config.AppConfig.DB.Name, config.AppConfig.DB.Collection)
	assert.NoError(t, err, "Failed to connect to test database")

	// Clear the existing collection to start fresh
	err = db.ClearCollection()
	assert.NoError(t, err, "Failed to clean up test database")

	return db
}

// - setupTestServer starts a test HTTP server for the registry service.
func setupTestServer(db *database.MongoDB) (*httptest.Server, *registry.Registry) {
	checkInterval := time.Duration(config.AppConfig.CheckIntervalMs) * time.Millisecond
	reg := registry.NewRegistry(db, checkInterval)

	router := mux.NewRouter()
	server.StartServer(reg, router, make(chan struct{}), "")

	return httptest.NewServer(router), reg
}

// Send a POST /register to the registry server with worker port in the body
func registerWorker(registryURL string, id string, httpport int, grpcport int, apiKey string) (*http.Response, error) {
	workerData := map[string]interface{}{
		"id":       id,
		"httpport": httpport,
		"grpcport": grpcport,
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

	ip := "127.0.0.1"
	resp, err := registerWorker(ts.URL, "workerID-test-1", 1234, 4321, config.AppConfig.APIKey)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify worker is registered in the database
	workers, err := db.GetAllWorkers()
	assert.NoError(t, err)
	assert.Len(t, workers, 1)
	assert.Equal(t, ip, workers[0]["host"])

	db.ClearCollection()
}

// TestIntegrationGetWorkerHealth tests retrieving the health status of a worker.
func TestIntegrationGetWorkerHealth(t *testing.T) {
	db := setupIntegrationDB(t)
	defer db.Disconnect()

	ts, reg := setupTestServer(db)
	defer ts.Close()

	address := "1.2.3.4"
	id := "workerID-test-2"
	reg.RegisterWorker(id, address, 1, 2) // register the server in the registry cache and DB

	req, err := http.NewRequest("GET", ts.URL+"/worker/health?address="+address+":1", nil)
	assert.NoError(t, err)

	// Include API Key in the request header
	req.Header.Set("X-API-Key", config.AppConfig.APIKey)

	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var response server.HealthResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "healthy", response.HealthStatus)

	db.ClearCollection()
}

// TestIntegrationGetHealthyWorkers tests retrieving all healthy workers.
func TestIntegrationGetHealthyWorkers(t *testing.T) {
	db := setupIntegrationDB(t)
	defer db.Disconnect()

	ts, reg := setupTestServer(db)
	defer ts.Close()

	address := "1.2.3.4"
	id := "workerID-test-3"
	reg.RegisterWorker(id, address, 1, 2) // register the server in the registry cache

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
	assert.Equal(t, address+":1", addresses[0])

	db.ClearCollection()
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
	host, portstr, err := middleware.GetHostAndPortFromURL(address)
	assert.NoError(t, err)

	port := portstr
	log.Printf("MockWorker IP : %s Port : %d", host, port)
	reg.RegisterWorker("workerID-test-4", host, port, 2) // register the server in the registry cache. Injection in the DB will fail as the port is random.

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

	var response server.HealthResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "healthy", response.HealthStatus)

	db.ClearCollection()
}
