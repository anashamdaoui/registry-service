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
	if [ ! -f $(CONFIG_FILE) ]; then echo "Config file missing"; exit 1; fi
	go build -o bin/registry-service cmd/main.go

test-unit:
	@echo "Running unit tests..."
	go test -v ./test/unit

test-integration:
	@echo "Running integration tests..."
	go test -v ./test/integration

clean:
	@echo "Cleaning up..."
	rm -rf bin

docker-build: build
	docker build -t $(DOCKER_IMAGE_NAME) .

docker-run:
	docker run -p 9080:$(SERVER_PORT) --name $(DOCKER_CONTAINER_NAME) -v /etc/localtime:/etc/localtime:ro -v /etc/timezone:/etc/timezone:ro $(DOCKER_IMAGE_NAME)

run: build
	./bin/registry-service