package registry

import (
	"log"
	"net/http"
	"registry-service/internal/config"
	"sync"
	"time"
)

type Registry struct {
	mutex   sync.Mutex
	workers map[string]*Worker
}

func logDetail(message string) {
	if config.AppConfig.LogLevel == "DEBUG" {
		log.Println(message)
	}
}

func NewRegistry() *Registry {
	r := &Registry{
		workers: make(map[string]*Worker),
	}
	r.LoadWorkers()
	return r
}

func (r *Registry) RegisterWorker(address string) {
	logDetail("Starting RegisterWorker...")
	r.mutex.Lock()
	r.workers[address] = &Worker{
		Address:         address,
		IsHealthy:       true,
		LastHealthCheck: time.Now(),
	}
	r.mutex.Unlock()
	logDetail("Worker registered, saving workers...")
	r.SaveWorkers()
	logDetail("Completed RegisterWorker.")
}

func (r *Registry) UpdateHealth(address string, isHealthy bool) {
	logDetail("Starting UpdateHealth...")
	r.mutex.Lock()
	if worker, exists := r.workers[address]; exists {
		logDetail("Updating health status for worker at address: " + address)
		worker.IsHealthy = isHealthy
		worker.LastHealthCheck = time.Now()
		logDetail("Worker health updated at address: " + address)
	}
	r.mutex.Unlock()
	logDetail("Saving workers after health update...")
	r.SaveWorkers()
	logDetail("Completed UpdateHealth.")
}

func (r *Registry) CheckAllWorkers() {
	logDetail("Starting CheckAllWorkers...")
	r.mutex.Lock()
	workers := r.workers
	r.mutex.Unlock()

	for address := range workers {
		log.Printf("Checking health of worker at address %s", address)
		resp, err := http.Get(address + "/healthcheck")
		isHealthy := err == nil && resp.StatusCode == http.StatusOK
		r.UpdateHealth(address, isHealthy)
	}
	logDetail("Completed CheckAllWorkers.")
}

func (r *Registry) GetWorker(address string) (*Worker, bool) {
	logDetail("Starting GetWorker...")
	r.mutex.Lock()
	defer r.mutex.Unlock()
	worker, exists := r.workers[address]
	logDetail("Completed GetWorker.")
	return worker, exists
}

func (r *Registry) GetHealthyWorkers() []*Worker {
	logDetail("Starting GetHealthyWorkers...")
	r.mutex.Lock()
	defer r.mutex.Unlock()
	var healthyWorkers []*Worker
	for _, worker := range r.workers {
		if worker.IsHealthy {
			healthyWorkers = append(healthyWorkers, worker)
		}
	}
	logDetail("Completed GetHealthyWorkers.")
	return healthyWorkers
}
