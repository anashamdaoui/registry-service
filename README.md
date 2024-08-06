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
  "server_port": "8080",
  "api_key": "your_api_key_here"
}
```
- log_level: Defines the verbosity of logs. Set to "DEBUG" for detailed logging.
- server_port: The port on which the registry service will run.
- api_key: Defines the API token to be added in the Authorization header when communicating with the service through the API.

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
The Dockerfile is defined to expose the registry on a specific port.
The port exposed in the Docker is the same the registry server listens to.

1. Build the Docker image:
```bash
make docker-build
```

2. Run the Docker container
```bash
make docker-run
```
