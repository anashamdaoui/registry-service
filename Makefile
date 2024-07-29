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

