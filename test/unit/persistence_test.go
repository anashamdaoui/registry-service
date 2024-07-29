package unit

import (
	"os"
	"registry-service/internal/registry"
	"testing"
)

func TestSaveWorkers(t *testing.T) {
	t.Log("Starting TestSaveWorkers")
	file := "workers.json"
	os.Remove(file)

	reg := registry.NewRegistry()
	address := "http://worker1:8080"

	t.Log("Registering worker...")
	reg.RegisterWorker(address)

	t.Log("Saving workers...")
	reg.SaveWorkers()

	t.Log("Checking if the file exists...")
	if _, err := os.Stat(file); os.IsNotExist(err) {
		t.Fatalf("Expected persistence file to exist")
	}

	t.Log("Completed TestSaveWorkers")
}

func TestLoadWorkers(t *testing.T) {
	t.Log("Starting TestLoadWorkers")
	file := "workers.json"
	os.Remove(file)

	reg := registry.NewRegistry()
	address := "http://worker1:8080"

	t.Log("Registering worker and saving...")
	reg.RegisterWorker(address)
	reg.SaveWorkers()

	t.Log("Loading workers into a new registry instance...")
	reg2 := registry.NewRegistry()
	reg2.LoadWorkers()

	t.Log("Retrieving worker from new registry instance...")
	worker, exists := reg2.GetWorker(address)
	if !exists {
		t.Fatalf("Expected worker to exist after loading from file")
	}

	t.Log("Checking worker address...")
	if worker.Address != address {
		t.Errorf("Expected address %s, got %s", address, worker.Address)
	}

	t.Log("Removing persistence file...")
	os.Remove(file)
	t.Log("Completed TestLoadWorkers")
}
