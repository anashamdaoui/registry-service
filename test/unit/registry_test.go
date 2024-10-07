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

	ip := "172.3.4.5"
	reg.RegisterWorker("ID12342", ip, 8080, 5050)

	// Assert that the worker is registered in memory
	_, exists := reg.GetWorkerHealth(ip + ":8080")
	assert.True(t, exists, "Worker should exist in registry")

	// Assert that the worker is registered in the database
	dbWorkers, err := db.GetAllWorkers()
	assert.NoError(t, err, "Error retrieving workers from database")
	assert.Len(t, dbWorkers, 1, "There should be one worker in the database")
	assert.Equal(t, ip, dbWorkers[0]["host"], "Worker address in DB should match")
}

// TestUpdateHealth:
// Tests the ability to update a worker's health status in-memory and in the database.
func TestUpdateHealth(t *testing.T) {
	db := setupTestDB(t)
	defer db.Disconnect()

	checkInterval := time.Duration(config.AppConfig.CheckIntervalMs) * time.Millisecond
	reg := registry.NewRegistry(db, checkInterval)

	ip := "192.178.3.4"
	reg.RegisterWorker("ID1234", ip, 3003, 4500)

	// Update health status to false
	reg.UpdateHealth("ID1234", false)

	// Assert that the worker's health status is updated in memory
	_, exists := reg.GetWorkerHealth(ip + ":3003")
	assert.True(t, exists, "Worker should exist in registry")

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

	ip := "178.36.90.66"
	reg.RegisterWorker("ID1234", ip, 3000, 556754)

	// Retrieve the worker using the registry
	_, exists := reg.GetWorkerHealth(ip + ":3000")
	assert.True(t, exists, "Worker should exist in registry")
}

// Ensures healthy workers are retrieved.
func TestGetHealthyWorkersAddress(t *testing.T) {
	db := setupTestDB(t)
	defer db.Disconnect()

	checkInterval := time.Duration(config.AppConfig.CheckIntervalMs) * time.Millisecond
	reg := registry.NewRegistry(db, checkInterval)

	address1 := "198.36.3.4"
	address2 := "1.2.3.4"

	reg.RegisterWorker("ID1", address1, 8080, 6767)
	reg.RegisterWorker("ID2", address2, 6787, 773753)

	// Retrieve healthy workers
	urls := reg.GetHealthyWorkersURL()

	assert.Len(t, urls, 2, "There should be 2 healthy workers")
	assert.Equal(t, address1+":8080", urls[0], "Healthy worker address1 should match")
	assert.Equal(t, address2+":6787", urls[1], "Healthy worker address2 should match")
}

// TestLoadWorkersFromDB:
// Verifies that workers stored in the database are correctly loaded into memory on startup.
func TestLoadWorkersFromDB(t *testing.T) {
	db := setupTestDB(t)
	defer db.Disconnect()

	ip := "187.3.4.5"
	id := "ID1234"
	if err := db.InsertWorker(id, ip, 8763, 9789); err != nil {
		t.Fatalf("Failed to insert worker into database: %v", err)
	}

	checkInterval := time.Duration(config.AppConfig.CheckIntervalMs) * time.Millisecond
	reg := registry.NewRegistry(db, checkInterval)

	// Assert that the worker is loaded from the database into memory
	_, exists := reg.GetWorkerHealth(ip + ":8763")
	assert.True(t, exists, "Worker should be loaded from database")
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
		addresses[i] = "1.2.3." + strconv.Itoa(i)
		wg.Add(1)
		go func(address string) {
			defer wg.Done()
			id := "ID" + strconv.Itoa(i)
			reg.RegisterWorker(id, address, 1, 2)
		}(addresses[i])
	}

	wg.Wait()

	// Verify all workers are registered
	for _, address := range addresses {
		_, exists := reg.GetWorkerHealth(address + ":1")
		assert.True(t, exists, "Worker should be registered")
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
		address := "1.2.3." + strconv.Itoa(i)
		id := "ID" + strconv.Itoa(i)
		reg.RegisterWorker(id, address, 3333, 44444)
	}

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		id := "ID" + strconv.Itoa(i)
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			reg.UpdateHealth(id, false)
		}(id)
	}

	wg.Wait()

	// Verify all workers are marked unhealthy
	for i := 0; i < numWorkers; i++ {
		address := "1.2.3." + strconv.Itoa(i)
		_, exists := reg.GetWorkerHealth(address + ":3333")
		assert.True(t, exists, "Worker should be found")
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
		address := "1.2.3." + strconv.Itoa(i)
		id := "ID" + strconv.Itoa(i)
		wg.Add(1)
		go func(address string) {
			defer wg.Done()
			reg.RegisterWorker(id, address, 1, 2)
		}(address)
	}

	// Concurrently update health status
	for i := 0; i < numWorkers; i++ {
		id := "ID" + strconv.Itoa(i)
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			reg.UpdateHealth(id, false)
		}(id)
	}

	// Concurrently get worker status
	for i := 0; i < numWorkers; i++ {
		address := "1.2.3." + strconv.Itoa(i)
		wg.Add(1)
		go func(address string) {
			defer wg.Done()
			_, exists := reg.GetWorkerHealth(address + ":1")
			assert.True(t, exists, "Worker should be found")
		}(address)
	}

	wg.Wait()
}
