# Variables
DOCKER_IMAGE_NAME = registry-service
DOCKER_CONTAINER_NAME = registry-service-container
CONFIG_FILE = internal/config/config.json

# Extract the server port from config.json
SERVER_PORT = $(shell jq -r '.server_port' $(CONFIG_FILE))

.PHONY: all build test clean

all: build test

build:
	@echo "Building the project..."
	go build -o bin/registry-service cmd/main.go

test:
	@echo "Running unit tests..."
	go test -v ./test/unit
	@echo "Running integration tests..."
	go test -v ./test/integration

clean:
	@echo "Cleaning up..."
	rm -rf bin

docker-build: build
	docker build -t $(DOCKER_IMAGE_NAME) .

docker-run: 
	docker run -p $(SERVER_PORT):8080 --name $(DOCKER_CONTAINER_NAME) $(DOCKER_IMAGE_NAME)

run: build
	if [ ! -f $(CONFIG_FILE) ]; then echo "Config file missing"; exit 1; fi
	./bin/registry-service