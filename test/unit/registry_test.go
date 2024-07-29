package unit

import (
	"log"
	"registry-service/internal/registry"
	"testing"
)

func TestRegisterWorker(t *testing.T) {
	log.Println("Starting TestRegisterWorker")
	reg := registry.NewRegistry()

	address := "http://worker1:8080"
	log.Println("Registering worker...")
	reg.RegisterWorker(address)

	log.Println("Verifying worker registration...")
	if worker, exists := reg.GetWorker(address); !exists {
		t.Fatalf("Expected worker to be registered")
	} else {
		log.Printf("Worker found: %+v", worker)
	}

	log.Println("TestRegisterWorker completed successfully.")
}

func TestUpdateHealth(t *testing.T) {
	log.Println("Starting TestUpdateHealth")
	reg := registry.NewRegistry()

	address := "http://worker1:8080"
	log.Println("Registering worker...")
	reg.RegisterWorker(address)

	log.Println("Updating worker health status...")
	reg.UpdateHealth(address, false)

	log.Println("Verifying worker health update...")
	if worker, exists := reg.GetWorker(address); !exists {
		t.Fatalf("Expected worker to be found")
	} else if worker.IsHealthy {
		t.Fatalf("Expected worker to be unhealthy")
	} else {
		log.Printf("Worker health updated: %+v", worker)
	}

	log.Println("TestUpdateHealth completed successfully.")
}
