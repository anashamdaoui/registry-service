package registry

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"registry-service/internal/config"
	"registry-service/internal/database"
	"registry-service/internal/middleware"
	"registry-service/internal/observability"
	"strconv"
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
		id := w["id"].(string)
		host := w["host"].(string)
		httpport := w["http_port"].(int32) // values are stored as int32
		grpcport := w["grpc_port"].(int32) // values are stored as int32
		isHealthy := w["is_healthy"].(bool)
		lastHealthCheck := w["last_health_check"].(primitive.DateTime).Time()

		r.workers[id] = &Worker{
			Host:            host,
			HTTPPort:        httpport,
			GRPCPort:        grpcport,
			IsHealthy:       isHealthy,
			LastHealthCheck: lastHealthCheck,
		}
		fmt.Printf("REGISTRY CACHE (loadWorkersFromDB): id %s %+v\n", id, r)
	}
}

// startHealthCheckLoop runs the health check loop at a configured interval.
func (r *Registry) startHealthCheckLoop() {
	logger := middleware.GetLogger()
	ticker := time.NewTicker(r.checkInterval)
	defer ticker.Stop()
	fmt.Printf("REGISTRY CACHE (startHealthCheckLoop): %+v\n", r)

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
func (r *Registry) RegisterWorker(id string, host string, httpPort int32, grpcPort int32) {
	r.mutex.Lock()
	fmt.Printf("REGISTRY CACHE (RegisterWorker): %+v\n", r)
	defer r.mutex.Unlock()

	logger := middleware.GetLogger()

	logger.Debug("", "Registring Worker with IP %s HTTP port %d GRPC port %d", host, httpPort, grpcPort)

	// Use the worker ID as mapping key
	worker, exists := r.workers[id]
	if !exists {
		logger.Debug("", "Worker cache miss, insert in Cache and DB")
		worker = &Worker{Host: host, HTTPPort: httpPort, GRPCPort: grpcPort, IsHealthy: true, LastHealthCheck: time.Now()}
		r.workers[id] = worker
		if err := r.db.InsertWorker(id, host, httpPort, grpcPort); err != nil {
			logger.Info("", "Failed to insert worker into database: %v", err)
		}
	} else {
		logger.Debug("", "Worker cache match, update health status in Cache and DB")
		worker.IsHealthy = true
		worker.LastHealthCheck = time.Now()
		if err := r.db.UpdateWorkerHealth(id, true); err != nil {
			logger.Info("", "Failed to update worker in database: %v", err)
		}
	}

	// Record the health status in Prometheus metrics
	url := middleware.GetURLFromHostPort(host, httpPort)
	observability.RecordWorkerHealth(id, url, true)
}

// UpdateHealth updates the health status of a worker
func (r *Registry) UpdateHealth(id string, isHealthy bool) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	logger := middleware.GetLogger()
	logger.Debug("", "UpdateHealth worker ID %s isHealthy %v", id, isHealthy)

	worker, exists := r.workers[id]
	if !exists {
		return
	}

	if worker.IsHealthy != isHealthy {
		worker.IsHealthy = isHealthy
		worker.LastHealthCheck = time.Now()
		if err := r.db.UpdateWorkerHealth(id, isHealthy); err != nil {
			logger.Info("Cache - ", "Failed to update worker in database: %v", err)
		}

		// Record the health status in Prometheus metrics
		url := middleware.GetURLFromHostPort(worker.Host, worker.HTTPPort)
		observability.RecordWorkerHealth(id, url, isHealthy)
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
	fmt.Printf("REGISTRY CACHE (CheckAllWorkers): %+v\n", workers)

	isHealthy := false
	for key := range workers {

		url := middleware.GetURLFromHostPort(workers[key].Host, workers[key].HTTPPort)
		logger.Debug("", "Checking health of worker at url: %s", url)

		retries := 4
		for i := 0; i < retries; i++ {
			isHealthy = getWorkerHealth(url+"/healthcheck", config.AppConfig.APIKey)
			if isHealthy {
				r.UpdateHealth(key, true)
				logger.Debug("", "Worker %s is healthy", url)
				break
			} else {
				// Log the error and retry
				str := ""
				if i < retries {
					str = " Retrying..."
				}
				logger.Debug("", "Try #%d: Error checking worker (%s) health.%s", i, url, str)
			}
			time.Sleep(100 * time.Millisecond) // Backoff
		}

		if !isHealthy {
			logger.Info("", "Worker %s is not healthy after retries. Removing it from cache and database.", url)
			r.RemoveWorker(key)
		}
	}
}

// RemoveWorker removes a worker from the cache and database
func (r *Registry) RemoveWorker(key string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	logger := middleware.GetLogger()

	logger.Debug("Cache - ", "Removing worker with id %s", key)
	delete(r.workers, key)
	if err := r.db.DeleteWorker(key); err != nil {
		logger.Info("DB - ", "Failed to delete worker from database: %v", err)
	}
}

// GetWorkerHealth retrieves a worker by its address.
func (r *Registry) GetWorkerHealth(url string) (ishealthy bool, found bool) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	logger := middleware.GetLogger()

	for key := range r.workers {
		host, port, _ := middleware.GetHostAndPortFromURL(url)
		if r.workers[key].Host == host && r.workers[key].HTTPPort == port {
			logger.Debug("Cache - ", "Get worker %s health: %v", url, true)
			return r.workers[key].IsHealthy, true
		}
	}

	logger.Debug("Cache - ", "Get worker %s health: Not found", url)

	return false, false
}

// GetHealthyWorkers retrieves all healthy workers.
func (r *Registry) GetHealthyWorkersURL() []string {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	logger := middleware.GetLogger()
	logger.Debug("Cache - ", "Starting GetHealthyWorkersURL...")

	// By design only healthy workers are kept in the cache and the DB.
	urls := make([]string, 0, len(r.workers))
	for key := range r.workers {
		host := r.workers[key].Host
		port := strconv.Itoa(int(r.workers[key].HTTPPort))
		url := net.JoinHostPort(host, port)
		urls = append(urls, url)
	}

	logger.Debug("Cache - ", "Completed GetHealthyWorkers.")

	return urls
}
