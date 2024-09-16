package registry

import (
	"log"
	"net/http"
	"registry-service/internal/config"
	"registry-service/internal/database"
	"registry-service/internal/middleware"
	"registry-service/internal/observability"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Registry struct {
	mutex           sync.Mutex
	workers         map[string]*Worker
	db              *database.MongoDB
	checkInterval   time.Duration
	stopHealthCheck chan struct{}
}

func NewRegistry(db *database.MongoDB, checkInterval time.Duration) *Registry {
	r := &Registry{
		workers:       make(map[string]*Worker),
		db:            db,
		checkInterval: checkInterval,
	}
	r.loadWorkersFromDB()
	go r.startHealthCheckLoop() // Start health check loop in the background
	return r
}

// Load workers from the database into memory
func (r *Registry) loadWorkersFromDB() {
	workers, err := r.db.GetAllWorkers()
	if err != nil {
		log.Fatalf("Failed to load workers from database: %v", err)
	}

	for _, w := range workers {
		address := w["address"].(string)
		isHealthy := w["is_healthy"].(bool)
		lastHealthCheck := w["last_health_check"].(primitive.DateTime).Time()

		r.workers[address] = &Worker{
			Address:         address,
			IsHealthy:       isHealthy,
			LastHealthCheck: lastHealthCheck,
		}
	}
}

// startHealthCheckLoop runs the health check loop at a configured interval.
func (r *Registry) startHealthCheckLoop() {
	logger := middleware.GetLogger()
	ticker := time.NewTicker(r.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.CheckAllWorkers()
		case <-r.stopHealthCheck:
			logger.Info("", "Stopping health check loop...")
			return
		}
	}
}

// StopHealthCheck stops the health check loop.
func (r *Registry) StopHealthCheck() {
	close(r.stopHealthCheck)
}

// Register a new worker
func (r *Registry) RegisterWorker(address string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	logger := middleware.GetLogger()

	logger.Debug("", "Registring Worker with address %v", address)

	worker, exists := r.workers[address]
	if !exists {
		logger.Debug("", "Worker cache miss, insert in Cache and DB")
		worker = &Worker{Address: address, IsHealthy: true, LastHealthCheck: time.Now()}
		r.workers[address] = worker
		if err := r.db.InsertWorker(address); err != nil {
			logger.Info("", "Failed to insert worker into database: %v", err)
		}
	} else {
		logger.Debug("", "Worker cache match, update health status in Cache and DB")
		worker.IsHealthy = true
		worker.LastHealthCheck = time.Now()
		if err := r.db.UpdateWorkerHealth(address, true); err != nil {
			logger.Info("", "Failed to update worker in database: %v", err)
		}
	}

	// Record the health status in Prometheus metrics
	observability.RecordWorkerHealth(address, true)
}

// UpdateHealth updates the health status of a worker
func (r *Registry) UpdateHealth(address string, isHealthy bool) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	logger := middleware.GetLogger()
	logger.Debug("", "UpdateHealth worker address %s isHealthy %v", address, isHealthy)

	worker, exists := r.workers[address]
	if !exists {
		return
	}

	if worker.IsHealthy != isHealthy {
		worker.IsHealthy = isHealthy
		worker.LastHealthCheck = time.Now()
		if err := r.db.UpdateWorkerHealth(address, isHealthy); err != nil {
			logger.Info("Cache - ", "Failed to update worker in database: %v", err)
		}

		// Record the health status in Prometheus metrics
		observability.RecordWorkerHealth(address, isHealthy)
	}
}

func getWorkerHealth(url string, apiKey string) bool {
	logger := middleware.GetLogger()

	// Create a new GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logger.Debug("", "Failed to create request GET %s : %s", url, err)
		return false
	}

	// Set the X-API-Key header
	req.Header.Set("X-API-Key", apiKey)

	// Send the request
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.Debug("", "Failed to create request GET %s : %s", url, err)
		return false
	}
	defer resp.Body.Close()

	// Compute health result
	isHealthy := resp.StatusCode == http.StatusOK

	return isHealthy
}

// CheckAllWorkers checks the health of all workers in the cache.
func (r *Registry) CheckAllWorkers() {
	logger := middleware.GetLogger()

	r.mutex.Lock()
	workers := r.workers
	r.mutex.Unlock()

	isHealthy := false
	for address := range workers {

		logger.Debug("", "Checking health of worker at address: %s", address)

		retries := 4
		for i := 0; i < retries; i++ {
			isHealthy = getWorkerHealth(address+"/healthcheck", config.AppConfig.APIKey)
			if isHealthy {
				r.UpdateHealth(address, true)
				logger.Debug("", "Worker %s is healthy", address)
				break
			} else {
				// Log the error and retry
				str := ""
				if i < retries {
					str = " Retrying..."
				}
				logger.Debug("", "Try #%d: Error checking worker (%s) health.%s", i, address, str)
			}
			time.Sleep(100 * time.Millisecond) // Backoff
		}

		if !isHealthy {
			logger.Info("", "Worker %s is not healthy after retries. Removing it from cache and database.", address)
			r.RemoveWorker(address)
		}
	}
}

// RemoveWorker removes a worker from the cache and database
func (r *Registry) RemoveWorker(address string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	logger := middleware.GetLogger()

	logger.Debug("Cache - ", "Removing worker with address %v", address)
	delete(r.workers, address)
	if err := r.db.DeleteWorker(address); err != nil {
		logger.Info("DB - ", "Failed to delete worker from database: %v", err)
	}
}

// GetWorker retrieves a worker by its address.
func (r *Registry) GetWorker(address string) (*Worker, bool) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	logger := middleware.GetLogger()
	logger.Debug("Cache - ", "Starting GetWorker...")

	worker, exists := r.workers[address]

	logger.Debug("Cache -", "Completed GetWorker.")

	return worker, exists
}

// GetHealthyWorkers retrieves all healthy workers.
func (r *Registry) GetHealthyWorkersAddress() []string {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	logger := middleware.GetLogger()
	logger.Debug("Cache - ", "Starting GetHealthyWorkers...")

	// By design only healthy workers are kept in the cache and the DB.
	addresses := make([]string, 0, len(r.workers))
	for addresse := range r.workers {
		addresses = append(addresses, addresse)
	}

	logger.Debug("Cache - ", "Completed GetHealthyWorkers.")

	return addresses
}
