# Variables
DOCKER_IMAGE_NAME = registry-service
DOCKER_CONTAINER_NAME = registry-service-container
CONFIG_FILE = internal/config/config.json

# Extract the server port from config.json
SERVER_PORT = $(shell jq -r '.server_port' $(CONFIG_FILE))

.PHONY: all build test clean

all: build test-unit test-integration

build:
	@echo "Building the project..."
	go build -o bin/registry-service cmd/main.go

test-unit:
	@echo Deploying a MongoDB Docker
	docker pull mongo
	docker run --name test-mongo -p 27017:27017 -d mongo
	docker ps | grep test-mongo
	@echo "Running unit tests..."
	go test -v -vet=all -failfast ./test/unit
	@echo shutting down MongoDB Docker
	docker stop test-mongo
	docker rm test-mongo

test-integration:
	@echo Deploying a MongoDB Docker
	docker pull mongo
	docker run --name test-mongo -p 27017:27017 -d mongo
	docker ps | grep test-mongo
	@echo "Running integration tests..."
	go test -v -vet=all -failfast ./test/integration
	@echo shutting down MongoDB Docker
	docker stop test-mongo
	docker rm test-mongo

clean:
	@echo "Cleaning up..."
	rm -rf bin

docker-build: build
	docker build -t $(DOCKER_IMAGE_NAME) .

docker-run:
	docker run -p 9080:$(SERVER_PORT) --name $(DOCKER_CONTAINER_NAME) -v /etc/localtime:/etc/localtime:ro -v /etc/timezone:/etc/timezone:ro $(DOCKER_IMAGE_NAME)

run: build
	./bin/registry-service