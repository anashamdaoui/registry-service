package registry

import (
	"encoding/json"
	"log"
	"os"
)

const persistenceFile = "workers.json"

// SaveWorkers persists the current state of workers to a file.
func (r *Registry) SaveWorkers() {
	logDetail("Starting SaveWorkers...")
	r.mutex.Lock()
	defer r.mutex.Unlock()

	logDetail("Copying workers map...")
	workersCopy := make(map[string]*Worker)
	for k, v := range r.workers {
		workersCopy[k] = v
	}

	logDetail("Marshalling workers data...")
	data, err := json.Marshal(workersCopy)
	if err != nil {
		log.Printf("Failed to marshal workers data: %v", err)
		return
	}

	logDetail("Writing workers data to file...")
	err = os.WriteFile(persistenceFile, data, 0644)
	if err != nil {
		log.Printf("Failed to write workers data to file: %v", err)
	}
	logDetail("Completed SaveWorkers.")
}

// LoadWorkers loads the worker state from a file.
func (r *Registry) LoadWorkers() {
	logDetail("Starting LoadWorkers...")

	if _, err := os.Stat(persistenceFile); os.IsNotExist(err) {
		log.Printf("Persistence file does not exist. Starting fresh.")
		return
	}

	logDetail("Reading workers data from file...")
	data, err := os.ReadFile(persistenceFile)
	if err != nil {
		log.Printf("Failed to read workers data from file: %v", err)
		return
	}

	logDetail("Unmarshalling workers data...")
	workersCopy := make(map[string]*Worker)
	err = json.Unmarshal(data, &workersCopy)
	if err != nil {
		log.Printf("Failed to unmarshal workers data: %v", err)
		return
	}

	logDetail("Copying data to workers map...")
	r.mutex.Lock()
	r.workers = workersCopy
	r.mutex.Unlock()
	logDetail("Completed LoadWorkers.")
}
