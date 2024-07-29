package registry

import (
	"log"
	"net/http"
	"sync"
	"time"
)

type Registry struct {
	mutex   sync.Mutex
	workers map[string]*Worker
}

func NewRegistry() *Registry {
	r := &Registry{
		workers: make(map[string]*Worker),
	}
	r.LoadWorkers() // Load workers from persistence on startup
	return r
}

func (r *Registry) RegisterWorker(address string) {
	log.Println("Starting RegisterWorker...")
	r.mutex.Lock()
	r.workers[address] = &Worker{
		Address:         address,
		IsHealthy:       true,
		LastHealthCheck: time.Now(),
	}
	r.mutex.Unlock()
	log.Println("Worker registered, saving workers...")
	r.SaveWorkers() // Save state after registering a worker
	log.Println("Completed RegisterWorker.")
}

func (r *Registry) UpdateHealth(address string, isHealthy bool) {
	log.Println("Starting UpdateHealth...")
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if worker, exists := r.workers[address]; exists {
		worker.IsHealthy = isHealthy
		worker.LastHealthCheck = time.Now()
	}
	r.SaveWorkers() // Save state after updating health
	log.Println("Completed UpdateHealth.")
}

func (r *Registry) CheckAllWorkers() {
	log.Println("Starting CheckAllWorkers...")
	r.mutex.Lock()
	workers := r.workers
	r.mutex.Unlock()

	for address, worker := range workers {
		log.Printf("Checking health of worker at address: %s", worker.Address)
		resp, err := http.Get("http://" + address + "/healthcheck")
		isHealthy := err == nil && resp.StatusCode == http.StatusOK
		r.UpdateHealth(address, isHealthy)
	}
	log.Println("Completed CheckAllWorkers.")
}

func (r *Registry) GetWorker(address string) (*Worker, bool) {
	log.Println("Starting GetWorker...")
	r.mutex.Lock()
	defer r.mutex.Unlock()
	worker, exists := r.workers[address]
	log.Println("Completed GetWorker.")
	return worker, exists
}

func (r *Registry) GetHealthyWorkers() []*Worker {
	log.Println("Starting GetHealthyWorkers...")
	r.mutex.Lock()
	defer r.mutex.Unlock()
	var healthyWorkers []*Worker
	for _, worker := range r.workers {
		if worker.IsHealthy {
			healthyWorkers = append(healthyWorkers, worker)
		}
	}
	log.Println("Completed GetHealthyWorkers.")
	return healthyWorkers
}
