# Use an official Go runtime as a parent image
FROM golang:1.22 AS builder

# Set the working directory inside the container
WORKDIR /app

# Set environment variables for cross-compilation
# Adjust these to match the target architecture, e.g., 'linux/arm64' or 'linux/amd64'
ENV GOOS=linux
ENV GOARCH=amd64
ENV CGO_ENABLED=0

# Copy the go.mod and go.sum to leverage Docker cache
COPY go.sum .
COPY go.mod .
RUN go mod download

# Copy the current directory contents into the container at /app. Security: avoid global pattern / recursive copy
COPY internal/ ./internal/
COPY cmd/ ./cmd

# Build the Go app with cross-compilation settings
RUN go build -ldflags="-s -w" -o registry-service ./cmd/...

# Use a smaller base image to run the compiled binary
FROM alpine:3.20
RUN apk --no-cache add ca-certificates && \
    addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /home/appuser

# Copy the binary and config file from the builder stage to the production image
COPY --from=builder /app/registry-service .
COPY --from=builder /app/internal/config/config.json ./internal/config/config.json

# Expose the port the app runs on. Must match the server_port defined in config.json
EXPOSE 8080

# Run the web service on container startup as USER
USER appuser
CMD ["./registry-service"]
