# Registry Service

The Registry Service is responsible for managing worker registration, health status, and maintaining a cache of healthy workers. This service provides HTTP endpoints for worker registration, health checks, and retrieving healthy workers.

### Features

- Register workers
- Check worker health status
- Retrieve a list of healthy workers
- Persist worker data to disk
- Load worker data from disk on startup

## Configuration

The configuration for the registry service is defined in a `config.json` file. Here is an example configuration:

```json
{
  "log_level": "DEBUG",
  "port": "8080"
}
```
- log_level: Defines the verbosity of logs. Set to "DEBUG" for detailed logging.
- port: The port on which the registry service will run.

### Endpoints

- `/register?address={worker_address}`: Register a new worker.
- `/worker/health/{address}`: Get the health status of a specific worker.
- `/workers/healthy`: Get a list of healthy workers.

### Makefile

The Makefile includes targets to build, test, and clean the project.

## Usage
To build, test, and clean the project, use the following commands:

## Build the Project
```bash
make build
```
This command compiles the Go code and creates an executable in the bin directory.

## Test the Project
```bash
make test
```
This command runs both unit tests and integration tests.

## Clean the Project
```bash
make clean
```

### Using Docker
1. Build the Docker image:
```bash
docker build -t registry-service .
```
2. Run the Docker container
```bash
docker run -p 8080:8080 registry-service
```
