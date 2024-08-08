package unit

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"registry-service/internal/config"
	"registry-service/internal/database"
	"registry-service/internal/middleware"
	"registry-service/internal/registry"

	"github.com/stretchr/testify/assert"
)

// Test Setup:
// - setupTestDB is used to initialize a clean database state before each test, ensuring that each test runs independently.
// - The init function ensures that configuration settings are loaded from the config.json file before tests are executed.
func init() {
	// Load the configuration from the config file
	config.LoadConfig("config.json")
	// Initialize the logger
	middleware.InitLogger(config.AppConfig.LogLevel)
}

// setupTestDB initializes the test database and clears any existing data
func setupTestDB(t *testing.T) *database.MongoDB {
	// Connect to the MongoDB instance
	db, err := database.NewMongoDB(config.AppConfig.DB.URI, config.AppConfig.DB.Name, config.AppConfig.DB.Collection)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Clear the existing collection to start fresh
	err = db.ClearCollection()
	if err != nil {
		t.Fatalf("Failed to clean up test database: %v", err)
	}

	return db
}

// TestRegisterWorker:
// Tests that a worker can be registered and persists in both in-memory cache and the database.
func TestRegisterWorker(t *testing.T) {
	db := setupTestDB(t)
	defer db.Disconnect()

	checkInterval := time.Duration(config.AppConfig.CheckIntervalMs) * time.Millisecond
	reg := registry.NewRegistry(db, checkInterval)

	address := "http://worker1:8080"
	reg.RegisterWorker(address)

	// Assert that the worker is registered in memory
	worker, exists := reg.GetWorker(address)
	assert.True(t, exists, "Worker should exist in registry")
	assert.Equal(t, address, worker.Address, "Worker address should match")

	// Assert that the worker is registered in the database
	dbWorkers, err := db.GetAllWorkers()
	assert.NoError(t, err, "Error retrieving workers from database")
	assert.Len(t, dbWorkers, 1, "There should be one worker in the database")
	assert.Equal(t, address, dbWorkers[0]["address"], "Worker address in DB should match")
}

// TestUpdateHealth:
// Tests the ability to update a worker's health status in-memory and in the database.
func TestUpdateHealth(t *testing.T) {
	db := setupTestDB(t)
	defer db.Disconnect()

	checkInterval := time.Duration(config.AppConfig.CheckIntervalMs) * time.Millisecond
	reg := registry.NewRegistry(db, checkInterval)

	address := "http://worker1:8080"
	reg.RegisterWorker(address)

	// Update health status to false
	reg.UpdateHealth(address, false)

	// Assert that the worker's health status is updated in memory
	worker, exists := reg.GetWorker(address)
	assert.True(t, exists, "Worker should exist in registry")
	assert.False(t, worker.IsHealthy, "Worker should be unhealthy")

	// Assert that the worker's health status is updated in the database
	dbWorkers, err := db.GetAllWorkers()
	assert.NoError(t, err, "Error retrieving workers from database")
	assert.False(t, dbWorkers[0]["is_healthy"].(bool), "Worker in DB should be unhealthy")
}

// TestGetWorker:
// Verifies retrieval of a worker by address.
func TestGetWorker(t *testing.T) {
	db := setupTestDB(t)
	defer db.Disconnect()

	checkInterval := time.Duration(config.AppConfig.CheckIntervalMs) * time.Millisecond
	reg := registry.NewRegistry(db, checkInterval)

	address := "http://worker1:8080"
	reg.RegisterWorker(address)

	// Retrieve the worker using the registry
	worker, exists := reg.GetWorker(address)
	assert.True(t, exists, "Worker should exist in registry")
	assert.Equal(t, address, worker.Address, "Worker address should match")
}

// TestGetHealthyWorkers:
// Ensures only healthy workers are retrieved.
func TestGetHealthyWorkers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Disconnect()

	checkInterval := time.Duration(config.AppConfig.CheckIntervalMs) * time.Millisecond
	reg := registry.NewRegistry(db, checkInterval)

	address1 := "http://worker1:8080"
	address2 := "http://worker2:8080"

	reg.RegisterWorker(address1)
	reg.RegisterWorker(address2)

	// Mark address2 as unhealthy
	reg.UpdateHealth(address2, false)

	// Retrieve healthy workers
	healthyWorkers := reg.GetHealthyWorkers()

	assert.Len(t, healthyWorkers, 1, "There should be one healthy worker")
	assert.Equal(t, address1, healthyWorkers[0].Address, "Healthy worker address should match")
}

// TestLoadWorkersFromDB:
// Verifies that workers stored in the database are correctly loaded into memory on startup.
func TestLoadWorkersFromDB(t *testing.T) {
	db := setupTestDB(t)
	defer db.Disconnect()

	address := "http://worker1:8080"
	if err := db.InsertWorker(address); err != nil {
		t.Fatalf("Failed to insert worker into database: %v", err)
	}

	checkInterval := time.Duration(config.AppConfig.CheckIntervalMs) * time.Millisecond
	reg := registry.NewRegistry(db, checkInterval)

	// Assert that the worker is loaded from the database into memory
	worker, exists := reg.GetWorker(address)
	assert.True(t, exists, "Worker should be loaded from database")
	assert.Equal(t, address, worker.Address, "Worker address should match")
}

// Concurency tests
// TestConcurrentRegistration Simulates concurrent worker registrations to ensure no race conditions occur when registering multiple workers simultaneously.
func TestConcurrentRegistration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Disconnect()

	checkInterval := time.Duration(config.AppConfig.CheckIntervalMs) * time.Millisecond
	reg := registry.NewRegistry(db, checkInterval)

	var wg sync.WaitGroup
	numWorkers := 100
	addresses := make([]string, numWorkers)

	for i := 0; i < numWorkers; i++ {
		addresses[i] = "http://worker" + strconv.Itoa(i) + ":8080"
		wg.Add(1)
		go func(address string) {
			defer wg.Done()
			reg.RegisterWorker(address)
		}(addresses[i])
	}

	wg.Wait()

	// Verify all workers are registered
	for _, address := range addresses {
		worker, exists := reg.GetWorker(address)
		assert.True(t, exists, "Worker should be registered")
		assert.Equal(t, address, worker.Address, "Worker address should match")
	}
}

// TestConcurrentHealthUpdates Verifies that concurrent health status updates do not conflict or produce errors.
func TestConcurrentHealthUpdates(t *testing.T) {
	db := setupTestDB(t)
	defer db.Disconnect()

	checkInterval := time.Duration(config.AppConfig.CheckIntervalMs) * time.Millisecond
	reg := registry.NewRegistry(db, checkInterval)

	// Register workers first
	numWorkers := 100
	for i := 0; i < numWorkers; i++ {
		address := "http://worker" + strconv.Itoa(i) + ":8080"
		reg.RegisterWorker(address)
	}

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		address := "http://worker" + strconv.Itoa(i) + ":8080"
		wg.Add(1)
		go func(address string) {
			defer wg.Done()
			reg.UpdateHealth(address, false)
		}(address)
	}

	wg.Wait()

	// Verify all workers are marked unhealthy
	for i := 0; i < numWorkers; i++ {
		address := "http://worker" + strconv.Itoa(i) + ":8080"
		worker, exists := reg.GetWorker(address)
		assert.True(t, exists, "Worker should be found")
		assert.False(t, worker.IsHealthy, "Worker should be unhealthy")
	}
}

// TestConcurrentGetWorker Simultaneously registers workers, updates their health status, and retrieves their status
// to ensure all operations can run concurrently without issues.
func TestConcurrentGetWorker(t *testing.T) {
	db := setupTestDB(t)
	defer db.Disconnect()

	checkInterval := time.Duration(config.AppConfig.CheckIntervalMs) * time.Millisecond
	reg := registry.NewRegistry(db, checkInterval)

	numWorkers := 100
	var wg sync.WaitGroup

	// Concurrently register workers
	for i := 0; i < numWorkers; i++ {
		address := "http://worker" + strconv.Itoa(i) + ":8080"
		wg.Add(1)
		go func(address string) {
			defer wg.Done()
			reg.RegisterWorker(address)
		}(address)
	}

	// Concurrently update health status
	for i := 0; i < numWorkers; i++ {
		address := "http://worker" + strconv.Itoa(i) + ":8080"
		wg.Add(1)
		go func(address string) {
			defer wg.Done()
			reg.UpdateHealth(address, false)
		}(address)
	}

	// Concurrently get worker status
	for i := 0; i < numWorkers; i++ {
		address := "http://worker" + strconv.Itoa(i) + ":8080"
		wg.Add(1)
		go func(address string) {
			defer wg.Done()
			worker, exists := reg.GetWorker(address)
			if exists {
				assert.Equal(t, address, worker.Address, "Worker address should match")
			}
		}(address)
	}

	wg.Wait()
}
