package registry

import (
	"log"
	"net/http"
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

		// Record the health status in Prometheus metrics
		observability.RecordWorkerHealth(address, isHealthy)
	}
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

// CheckAllWorkers checks the health of all workers in the cache.
func (r *Registry) CheckAllWorkers() {
	logger := middleware.GetLogger()

	r.mutex.Lock()
	workers := r.workers
	r.mutex.Unlock()

	for address := range workers {
		resp, err := http.Get(address + "/healthcheck")
		isHealthy := err == nil && resp.StatusCode == http.StatusOK

		if err != nil || resp.StatusCode != http.StatusOK {
			// Log the error and implement a retry mechanism
			logger.Debug("", "Error checking health for worker %s: %v. Retrying...", address, err)

			// Retry logic
			retries := 3
			for i := 0; i < retries; i++ {
				time.Sleep(100 * time.Millisecond) // Backoff
				resp, err = http.Get(address + "/healthcheck")
				isHealthy = err == nil && resp.StatusCode == http.StatusOK
				if isHealthy {
					break
				}
			}
		}

		r.UpdateHealth(address, isHealthy)
		logger.Debug("", "Worker %s health check result: %v", address, isHealthy)

		if isHealthy {
			logger.Debug("", "Worker %s is healthy", address)
			r.UpdateHealth(address, true)
		} else {
			logger.Info("", "Worker %s is not healthy after retries. Removing from cache and database.", address)
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

// GetWorker retrieves a worker by its address.
func (r *Registry) GetWorker(address string) (*Worker, bool) {
	logger := middleware.GetLogger()

	logger.Debug("Cache - ", "Starting GetWorker...")
	r.mutex.Lock()
	defer r.mutex.Unlock()
	worker, exists := r.workers[address]
	logger.Debug("Cache -", "Completed GetWorker.")
	return worker, exists
}

// GetHealthyWorkers retrieves all healthy workers.
func (r *Registry) GetHealthyWorkers() []*Worker {
	logger := middleware.GetLogger()

	logger.Debug("Cache - ", "Starting GetHealthyWorkers...")
	r.mutex.Lock()
	defer r.mutex.Unlock()

	var healthyWorkers []*Worker
	for _, worker := range r.workers {
		if worker.IsHealthy {
			healthyWorkers = append(healthyWorkers, worker)
		}
	}
	logger.Debug("Cache - ", "Completed GetHealthyWorkers.")
	return healthyWorkers
}
